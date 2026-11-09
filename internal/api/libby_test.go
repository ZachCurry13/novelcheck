package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// A fake OverDrive: library "mylib", with Dune on 3 holds and a look-alike first.
func fakeOverDrive(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/libraries/mylib":
			fmt.Fprint(w, `{"name": "My Public Library"}`)
		case r.URL.Path == "/libraries/mylib/media" && r.URL.Query().Get("mediaTypes") == "ebook":
			fmt.Fprint(w, `{"items": [
			 {"id": "1", "title": "Dune Messiah", "firstCreatorName": "Frank Herbert", "isAvailable": true, "isHoldable": true, "ownedCopies": 5, "availableCopies": 2},
			 {"id": "2", "title": "Dune", "firstCreatorName": "Frank Herbert", "isAvailable": false, "isHoldable": true, "ownedCopies": 10, "availableCopies": 0,
			  "holdsCount": 3, "estimatedWaitDays": 14, "formats": [{"id": "ebook-overdrive"}, {"id": "ebook-kindle"}]}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLibbyAvailability(t *testing.T) {
	od := fakeOverDrive(t)
	srv, st := setupWith(t, func(s *api.Server) { s.Libby.Base = od.URL })
	admin := login(t, srv, "admin", "adminpass1")
	id, _ := st.UpsertBook("Dune", "Frank Herbert", "", "")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	_ = st.AddCopy(cal, id, "/c/d.epub", "epub", "1")
	path := fmt.Sprintf("/api/books/%d/libby", id)

	if _, out := admin.do("GET", path, nil, false); out["configured"] != false {
		t.Fatalf("no library yet: %v", out)
	}
	if res, _ := admin.do("PUT", "/api/me/libby", map[string]string{"library": "not a key!"}, true); res.StatusCode != 400 {
		t.Fatalf("a bad key: %d", res.StatusCode)
	}
	if res, _ := admin.do("PUT", "/api/me/libby", map[string]string{"library": "nolib"}, true); res.StatusCode != 502 {
		t.Fatalf("an unknown library: %d", res.StatusCode)
	}
	if res, out := admin.do("PUT", "/api/me/libby", map[string]string{"library": "https://MyLib.overdrive.com/"}, true); res.StatusCode != 200 || out["library"] != "mylib" || out["name"] != "My Public Library" {
		t.Fatalf("set library: %d %v", res.StatusCode, out)
	}
	_, out := admin.do("GET", path, nil, false)
	a, _ := out["availability"].(map[string]any)
	if a == nil || a["found"] != true || a["holds"].(float64) != 3 || a["wait_days"].(float64) != 14 || a["available"] != false ||
		a["kindle"] != true || !strings.HasSuffix(a["url"].(string), "/media/2") || !strings.Contains(a["url"].(string), "mylib.overdrive.com") {
		t.Fatalf("availability picks the exact title, not the look-alike: %v", out)
	}
	_, batch := admin.do("POST", "/api/libby/check", map[string]any{"ids": []int64{id, 999999}}, true)
	books, _ := batch["books"].(map[string]any)
	if batch["configured"] != true || len(books) != 1 || books[fmt.Sprint(id)].(map[string]any)["holds"].(float64) != 3 {
		t.Fatalf("batch check (unknown books are left out): %v", batch)
	}
	other, _ := st.UpsertBook("A Book The Library Lacks", "Nobody", "", "")
	_ = st.AddCopy(cal, other, "/c/o.epub", "epub", "2")
	if _, out := admin.do("GET", fmt.Sprintf("/api/books/%d/libby", other), nil, false); out["availability"].(map[string]any)["found"] != false {
		t.Fatalf("not at the library: %v", out)
	}
}

// Two libraries: each book is looked up at both and the best answer wins; the audiobooks of books without
// one are checked in the background and kept, and the kept answers are served without asking again.
func TestLibbyTwoLibrariesAndAudioScan(t *testing.T) {
	od := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/libraries/mylib":
			fmt.Fprint(w, `{"name": "My Public Library"}`)
		case r.URL.Path == "/libraries/citylib":
			fmt.Fprint(w, `{"name": "City Library"}`)
		case r.URL.Path == "/libraries/mylib/media" && r.URL.Query().Get("mediaTypes") == "ebook":
			fmt.Fprint(w, `{"items": [{"id": "2", "title": "Dune", "firstCreatorName": "Frank Herbert", "isAvailable": false, "isHoldable": true, "ownedCopies": 10, "holdsCount": 3, "estimatedWaitDays": 14}]}`)
		case r.URL.Path == "/libraries/citylib/media" && r.URL.Query().Get("mediaTypes") == "audiobook":
			fmt.Fprint(w, `{"items": [{"id": "9", "title": "Dune", "firstCreatorName": "Frank Herbert", "isAvailable": true, "isHoldable": true, "ownedCopies": 4, "availableCopies": 1}]}`)
		case strings.HasPrefix(r.URL.Path, "/libraries/") && strings.HasSuffix(r.URL.Path, "/media"):
			fmt.Fprint(w, `{"items": []}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer od.Close()
	srv, st := setupWith(t, func(s *api.Server) { s.Libby.Base = od.URL })
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	dune, _ := st.UpsertBook("Dune", "Frank Herbert", "", "")
	_ = st.AddCopy(cal, dune, "/c/d.epub", "epub", "1")
	other, _ := st.UpsertBook("A Book Nobody Lends", "Nobody", "", "")
	_ = st.AddCopy(cal, other, "/c/o.epub", "epub", "2")
	for _, k := range []string{"mylib", "citylib"} {
		if res, _ := admin.do("PUT", "/api/me/libby", map[string]string{"library": k}, true); res.StatusCode != 200 {
			t.Fatalf("add %s: %d", k, res.StatusCode)
		}
	}
	_, libs := admin.do("GET", "/api/me/libby", nil, false)
	if l, _ := libs["libraries"].([]any); len(l) != 2 || libs["library"] != "mylib" {
		t.Fatalf("both libraries, the first for older code: %v", libs)
	}
	_, out := admin.do("GET", fmt.Sprintf("/api/books/%d/libby", dune), nil, false)
	au, _ := out["audio"].(map[string]any)
	a, _ := out["availability"].(map[string]any)
	if per, _ := out["libraries"].([]any); len(per) != 2 || a["library"] != "My Public Library" || au["library"] != "City Library" || au["available"] != true {
		t.Fatalf("the e-book from one library, the audiobook from the other: %v", out)
	}
	// The background check: found ones are kept, ones nobody lends are remembered as not found.
	if res, sc := admin.do("POST", "/api/me/libby/scan?wait=1", map[string]string{}, true); res.StatusCode != 200 || sc["checked"].(float64) != 2 {
		t.Fatalf("scan: %d %v", res.StatusCode, sc)
	}
	_, kept := admin.do("POST", "/api/libby/audio", map[string]any{"ids": []int64{dune, other}}, true)
	m, _ := kept["audio"].(map[string]any)
	row, _ := m[fmt.Sprint(dune)].(map[string]any)
	if len(m) != 1 || row["available"] != true || row["library"] != "City Library" || !strings.Contains(row["url"].(string), "citylib.overdrive.com") {
		t.Fatalf("only the audiobook a library lends is served: %v", kept)
	}
	if due := st.LibbyAudioDue(1, 10); len(due) != 0 {
		t.Fatalf("nothing is due again so soon: %v", due)
	}
	if _, lst := admin.do("GET", "/api/books?libby_audio=1", nil, false); lst["total"] != float64(1) {
		t.Fatalf("the Library can filter to books whose audiobook a library lends: %v", lst["total"])
	}
	if _, lst := admin.do("GET", "/api/books?libby_audio=now", nil, false); lst["total"] != float64(1) {
		t.Fatalf("...and available now: %v", lst["total"])
	}
	// Removing one library keeps the other; removing the last clears what was kept.
	if res, l := admin.do("DELETE", "/api/me/libby/citylib", nil, true); res.StatusCode != 200 || len(l["libraries"].([]any)) != 1 {
		t.Fatalf("remove one: %d %v", res.StatusCode, l)
	}
}
