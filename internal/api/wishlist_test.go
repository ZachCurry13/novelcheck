package api_test

import (
	"strconv"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestWishlist(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	lookedUp, _ := st.EnsureCatalog(store.LookedUpCatalog, "custom")
	calibre, _ := st.EnsureCatalog("Calibre Main", "calibre")
	zero := 0
	want, _ := st.UpsertBook("Shop Find", "A", "", "")
	_ = st.AddCopy(lookedUp, want, "lookup:Shop Find", "list", "")
	_ = st.SaveAnalysis(want, store.Analysis{SpiceLevel: &zero, Model: "gpt"})
	have, _ := st.UpsertBook("Already Here", "B", "", "")
	_ = st.AddCopy(calibre, have, "/c/h.epub", "epub", "1")

	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	_, _ = st.DB.Exec(`UPDATE users SET hide_unrated = 0 WHERE username = 'kid'`)
	kid := login(t, srv, "kid", "kidpass12")
	if res, out := kid.do("POST", "/api/books/"+itoa(want)+"/wish", map[string]string{"note": "for my birthday"}, true); res.StatusCode != 200 || out["wish"] == nil {
		t.Fatalf("add wish: %d %v", res.StatusCode, out)
	}
	if res, _ := kid.do("POST", "/api/books/"+itoa(want)+"/wish", nil, true); res.StatusCode != 409 {
		t.Fatalf("wishing twice: %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", "/api/books/"+itoa(have)+"/wish", nil, true); res.StatusCode != 400 {
		t.Fatalf("books we have can't be wished for: %d", res.StatusCode)
	}
	admin.do("POST", "/api/books/"+itoa(want)+"/wish", nil, true)

	// Kids see their own wishes; parents see everyone's and decide.
	if _, out := kid.do("GET", "/api/wishlist", nil, false); len(out["items"].([]any)) != 1 || out["manager"] != false {
		t.Fatalf("kid's list: %v", out)
	}
	_, out := admin.do("GET", "/api/wishlist", nil, false)
	items := out["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("parent's list: %v", items)
	}
	kidWish := int64(0)
	for _, it := range items {
		if m := it.(map[string]any); m["username"] == "kid" {
			kidWish = int64(m["id"].(float64))
			if m["note"] != "for my birthday" || m["status"] != "wanted" {
				t.Fatalf("kid's wish: %v", m)
			}
		}
	}
	if res, _ := kid.do("POST", "/api/wishlist/"+strconv.FormatInt(kidWish, 10)+"/approve", nil, true); res.StatusCode != 403 {
		t.Fatalf("kids can't approve: %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", "/api/wishlist/"+strconv.FormatInt(kidWish, 10)+"/approve", nil, true); res.StatusCode != 200 {
		t.Fatalf("approve: %d", res.StatusCode)
	}

	// Pending acquisition in Up Next, then the book arrives in Calibre.
	_ = st.Enqueue(1, want)
	if q, _ := st.ListQueue(1, false); len(q) != 1 || q[0].Owned {
		t.Fatalf("queued book not owned yet: %+v", q)
	}
	_ = st.AddCopy(calibre, want, "/c/w.epub", "epub", "2")
	if _, out = kid.do("GET", "/api/wishlist", nil, false); out["items"].([]any)[0].(map[string]any)["status"] != "acquired" {
		t.Fatalf("arrived books are marked acquired: %v", out)
	}
	if q, _ := st.ListQueue(1, false); !q[0].Owned {
		t.Fatal("now it's owned")
	}
}
