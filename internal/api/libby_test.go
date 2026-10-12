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
	other, _ := st.UpsertBook("A Book The Library Lacks", "Nobody", "", "")
	_ = st.AddCopy(cal, other, "/c/o.epub", "epub", "2")
	if _, out := admin.do("GET", fmt.Sprintf("/api/books/%d/libby", other), nil, false); out["availability"].(map[string]any)["found"] != false {
		t.Fatalf("not at the library: %v", out)
	}
}
