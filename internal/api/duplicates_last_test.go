package api_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// NovelCheck's list can be behind Calibre: a copy removed a moment ago is still on it. Removing "the other
// copy" then must be refused, because Calibre has only one left.
func TestNeverRemoveTheLastCopyCalibreHas(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	hobbit, _ := st.UpsertBook("The Hobbit", "J.R.R. Tolkien", "", "")
	_ = st.AddCopy(cal, hobbit, "/calibre/Tolkien/The Hobbit (10)/h.mobi", "MOBI", "10")
	_ = st.AddCopy(cal, hobbit, "/calibre/Tolkien/The Hobbit (11)/h.epub", "EPUB", "11")

	var removed atomic.Int32
	cs := fakeCalibre(t, map[string]string{"11": "The Hobbit"}, &removed) // #10 is already gone from Calibre
	admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": cs.URL}, true)

	res, out := admin.do("POST", "/api/admin/calibre/duplicates/remove", map[string]any{"remove": []string{"11"}, "later": true}, true)
	if res.StatusCode != 409 || removed.Load() != 0 || !strings.Contains(out["error"].(string), "last copy") {
		t.Fatalf("the last copy Calibre has must stay: %d %v (removed %d)", res.StatusCode, out, removed.Load())
	}
	// The copy that is already gone can still be "removed" (nothing happens), and the real one stays.
	res, out = admin.do("POST", "/api/admin/calibre/duplicates/remove", map[string]any{"remove": []string{"10"}, "later": true}, true)
	if res.StatusCode != 200 || removed.Load() != 0 || out["skipped"].(float64) != 1 {
		t.Fatalf("an entry already gone is skipped: %d %v (removed %d)", res.StatusCode, out, removed.Load())
	}
}

// When the copy that would stay is missing from the Content server but its file is in the library, the Content
// server is behind (books added with Calibre-Web): say so, with a notice, and remove nothing.
func TestContentServerBehindIsSaid(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	dir := t.TempDir()
	kept := filepath.Join(dir, "h.mobi")
	_ = os.WriteFile(kept, []byte("mobi"), 0o644)
	hobbit, _ := st.UpsertBook("The Hobbit", "J.R.R. Tolkien", "", "")
	_ = st.AddCopy(cal, hobbit, kept, "MOBI", "10")
	_ = st.AddCopy(cal, hobbit, filepath.Join(dir, "h.epub"), "EPUB", "11")

	var removed atomic.Int32
	cs := fakeCalibre(t, map[string]string{"11": "The Hobbit"}, &removed) // the Content server doesn't know #10 yet
	admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": cs.URL}, true)

	res, out := admin.do("POST", "/api/admin/calibre/duplicates/remove", map[string]any{"remove": []string{"11"}, "later": true}, true)
	if res.StatusCode != 409 || removed.Load() != 0 || !strings.Contains(out["error"].(string), "Content server is behind") {
		t.Fatalf("say the Content server is behind: %d %v", res.StatusCode, out)
	}
	items, _, _ := st.Notifications(10)
	found := false
	for _, n := range items {
		found = found || n.Source == "calibre-behind"
	}
	if !found {
		t.Fatalf("a 🔔 notice: %+v", items)
	}
}
