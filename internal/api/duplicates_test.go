package api_test

import (
	"strings"
	"sync/atomic"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestDuplicatesAPI(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	// "The Hobbit" imported twice (#10 MOBI only, #11 EPUB+AZW3), "Dune" once.
	hobbit, _ := st.UpsertBook("The Hobbit", "J.R.R. Tolkien", "", "")
	_ = st.AddCopy(cal, hobbit, "/calibre/Tolkien/The Hobbit (10)/h.mobi", "MOBI", "10")
	_ = st.AddCopy(cal, hobbit, "/calibre/Tolkien/The Hobbit (11)/h.epub", "EPUB", "11")
	_ = st.AddCopy(cal, hobbit, "/calibre/Tolkien/The Hobbit (11)/h.azw3", "AZW3", "11")
	dune, _ := st.UpsertBook("Dune", "Frank Herbert", "", "")
	_ = st.AddCopy(cal, dune, "/calibre/Herbert/Dune (12)/d.epub", "EPUB", "12")

	res, out := admin.do("GET", "/api/admin/calibre/duplicates", nil, false)
	groups, _ := out["groups"].([]any)
	if res.StatusCode != 200 || len(groups) != 1 || out["extra"].(float64) != 1 {
		t.Fatalf("duplicates: %d %v", res.StatusCode, out)
	}
	g := groups[0].(map[string]any)
	if g["title"] != "The Hobbit" || g["keep"] != "11" || len(g["entries"].([]any)) != 2 {
		t.Fatalf("group: %v", g)
	}

	// Formats show on the book and the Format filter finds duplicates/multi-format books.
	_, bl := admin.do("GET", "/api/books?format=dupes", nil, false)
	if bl["total"].(float64) != 1 {
		t.Fatalf("dupes filter: %v", bl)
	}
	b := bl["books"].([]any)[0].(map[string]any)
	if b["formats"] != "AZW3,EPUB,MOBI" || b["calibre_copies"].(float64) != 2 {
		t.Fatalf("formats: %v", b)
	}
	if _, bl = admin.do("GET", "/api/books?format=mobi", nil, false); bl["total"].(float64) != 1 {
		t.Fatalf("mobi filter: %v", bl)
	}

	var removed atomic.Int32
	cs := fakeCalibre(t, map[string]string{"10": "The Hobbit", "11": "The Hobbit", "12": "Dune"}, &removed)
	admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": cs.URL}, true)
	// Never every copy, never a book that isn't a duplicate.
	if res, out := admin.do("POST", "/api/admin/calibre/duplicates/remove", map[string]any{"remove": []string{"10", "11"}}, true); res.StatusCode != 400 || removed.Load() != 0 {
		t.Fatalf("removing all copies must be refused: %d %v", res.StatusCode, out)
	}
	if res, out := admin.do("POST", "/api/admin/calibre/duplicates/remove", map[string]any{"remove": []string{"12"}}, true); res.StatusCode != 409 || !strings.Contains(out["error"].(string), "Nothing was removed") {
		t.Fatalf("non-duplicate must be refused: %d %v", res.StatusCode, out)
	}
	res, out = admin.do("POST", "/api/admin/calibre/duplicates/remove", map[string]any{"remove": []string{"10"}}, true)
	if res.StatusCode != 200 || out["removed"].(float64) != 1 || removed.Load() != 1 {
		t.Fatalf("remove duplicate: %d %v", res.StatusCode, out)
	}

	// Editors can look but not remove.
	admin.do("POST", "/api/admin/users", map[string]string{"username": "mom", "password": "mompass123", "role": "editor"}, true)
	mom := login(t, srv, "mom", "mompass123")
	if res, _ := mom.do("GET", "/api/admin/calibre/duplicates", nil, false); res.StatusCode != 200 {
		t.Fatalf("editor list: %d", res.StatusCode)
	}
	if res, _ := mom.do("POST", "/api/admin/calibre/duplicates/remove", map[string]any{"remove": []string{"10"}}, true); res.StatusCode != 403 {
		t.Fatalf("editor remove: %d", res.StatusCode)
	}
}
