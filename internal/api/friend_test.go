package api_test

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// A friend's account sees Discover, events, her own library and her reading
// list, and nothing from the family's libraries, by any door.
func TestAFriendNeverSeesTheFamilyLibrary(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	if res, _ := admin.do("POST", "/api/admin/users", map[string]string{"username": "mom", "password": "mompass123", "role": "editor"}, true); res.StatusCode != 201 {
		t.Fatalf("mom: %d", res.StatusCode)
	}
	mom := login(t, srv, "mom", "mompass123")
	// A parent can make a friend's account.
	if res, out := mom.do("POST", "/api/admin/users", map[string]string{"username": "pal", "password": "palpass123", "role": "friend"}, true); res.StatusCode != 201 {
		t.Fatalf("friend: %d %v", res.StatusCode, out)
	}

	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	owned, _ := st.UpsertBook("Family Book", "Ann Author", "", "")
	_ = st.AddCopy(cal, owned, "/c/family.epub", "epub", "1")
	disc, _ := st.EnsureCatalog(store.DiscoverCatalog, "custom")
	hot, _ := st.UpsertBook("Hot New Book", "Bo Writer", "", "")
	_ = st.AddCopy(disc, hot, "", "list", "")
	both, _ := st.UpsertBook("Best Seller We Own", "Cy Penn", "", "")
	_ = st.AddCopy(disc, both, "", "list", "")
	_ = st.AddCopy(cal, both, "/c/best.epub", "epub", "2")
	evID, _, err := st.CreateEvent("Free Friday", "", "mom", "", []store.EventEntry{{Title: "Free One", Author: "Di Free", ASIN: "B000000001"}})
	if err != nil {
		t.Fatal(err)
	}
	var free int64
	_ = st.DB.Get(&free, `SELECT book_id FROM event_books WHERE event_id = ?`, evID)

	pal := login(t, srv, "pal", "palpass123")
	ids := func(path string) map[float64]bool {
		res, body := pal.do("GET", path, nil, false)
		if res.StatusCode != 200 {
			t.Fatalf("%s: %d", path, res.StatusCode)
		}
		out := map[float64]bool{}
		list, _ := body["books"].([]any)
		for _, b := range list {
			out[b.(map[string]any)["id"].(float64)] = true
		}
		return out
	}
	if got := ids("/api/books?event=" + itoa(evID)); !got[float64(free)] || len(got) != 1 {
		t.Fatalf("event books: %v", got)
	}
	if got := ids("/api/books?q=Book"); got[float64(owned)] || got[float64(both)] {
		t.Fatalf("a family book in search: %v", got)
	}
	for _, id := range []int64{owned, both} {
		for _, p := range []string{"/api/books/%d", "/api/books/%d/cover", "/api/books/%d/download", "/api/books/%d/listen"} {
			path := sprintf(p, id)
			if res, _ := pal.do("GET", path, nil, false); res.StatusCode == 200 {
				t.Fatalf("%s: %d", path, res.StatusCode)
			}
		}
	}
	if res, _ := pal.do("GET", sprintf("/api/books/%d", hot), nil, false); res.StatusCode != 200 {
		t.Fatalf("a Discover book: %d", res.StatusCode)
	}
	for _, p := range []string{"/api/readers", "/api/series", "/api/wishlist", "/api/queue", "/api/listening", "/api/me/opds", "/api/me/kosync", "/api/notifications"} {
		if res, _ := pal.do("GET", p, nil, false); res.StatusCode != http.StatusForbidden {
			t.Fatalf("%s: %d", p, res.StatusCode)
		}
	}
	if res, _ := pal.do("POST", sprintf("/api/books/%d/wish", hot), map[string]string{}, true); res.StatusCode != http.StatusForbidden {
		t.Fatalf("a wish (a request to buy): %d", res.StatusCode)
	}
	if res, cols := pal.doList("GET", "/api/collections"); res.StatusCode != 200 || len(cols) != 0 {
		t.Fatalf("collections: %d %v", res.StatusCode, cols)
	}
	if res, _ := pal.do("GET", "/api/discover", nil, false); res.StatusCode != 200 {
		t.Fatalf("discover: %d", res.StatusCode)
	}

	// Claiming an event book puts it in her own library, the only one she sees.
	if res, out := pal.do("POST", sprintf("/api/me/events/%d/books/%d/claim", evID, free), map[string]int{"catalog_id": int(cal)}, true); res.StatusCode != 200 {
		t.Fatalf("claim: %d %v", res.StatusCode, out)
	}
	res, libs := pal.doList("GET", "/api/catalogs")
	if res.StatusCode != 200 || len(libs) != 1 || libs[0]["name"] != "pal's books" || libs[0]["book_count"].(float64) != 1 {
		t.Fatalf("her libraries: %v", libs)
	}
	// Her Goodreads list goes to her library too, whatever library is named.
	if res, out := pal.do("POST", "/api/me/import", map[string]any{"catalog_id": cal, "books": []map[string]any{
		{"title": "A Book She Read", "author": "Ed Read", "format": "list", "read": true}}}, true); res.StatusCode != 200 {
		t.Fatalf("import: %d %v", res.StatusCode, out)
	}
	if got := ids("/api/books?q=She"); len(got) != 1 {
		t.Fatalf("her imported book: %v", got)
	}
	var inCalibre int
	_ = st.DB.Get(&inCalibre, `SELECT COUNT(*) FROM catalog_books WHERE catalog_id = ?`, cal)
	if inCalibre != 2 {
		t.Fatalf("the Calibre library was changed: %d books", inCalibre)
	}
	// The parents' versions stay closed to her, and hers to everyone else.
	if res, _ := pal.do("POST", sprintf("/api/events/%d/books/%d/claim", evID, free), map[string]int{}, true); res.StatusCode != http.StatusForbidden {
		t.Fatalf("the parents' claim: %d", res.StatusCode)
	}
	if res, _ := mom.do("POST", "/api/me/import", map[string]any{"books": []any{}}, true); res.StatusCode != http.StatusForbidden {
		t.Fatalf("a parent on the friend's import: %d", res.StatusCode)
	}
	// KOReader's doors (sync, catalog, files) refuse her even with a code.
	_, _ = st.DB.Exec(`UPDATE users SET kosync_code = 'code123' WHERE username = 'pal'`)
	sum := md5.Sum([]byte("code123"))
	if _, ok := st.KosyncUser("pal", hex.EncodeToString(sum[:])); ok {
		t.Fatal("KOReader sync let a friend in")
	}
}

func sprintf(format string, a ...any) string { return fmt.Sprintf(format, a...) }
