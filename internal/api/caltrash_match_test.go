package api_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The bin says, for each removed book, whether another copy of the same book is still in Calibre (an extra copy
// was removed) or none is (the book is missing from the library and is one to put back). The removed copy itself
// never counts, even before the library is read again.
func TestRecycleBinAgainstLibrary(t *testing.T) {
	lib := t.TempDir()
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.CalibreDir, s.Syncer.Dir = lib, lib })
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	kept, _ := st.UpsertBook("Dune", "Frank Herbert", "", "")
	_ = st.AddCopy(cal, kept, filepath.Join(lib, "Frank Herbert", "Dune (900)", "Dune.epub"), "epub", "900")
	gone, _ := st.UpsertBook("Emma", "Jane Austen", "", "")
	_ = st.AddCopy(cal, gone, filepath.Join(lib, "Jane Austen", "Emma (77)", "Emma.epub"), "epub", "77") // the removed copy, not yet re-read
	entry := func(id, title, author string) {
		dir := filepath.Join(lib, ".caltrash", "b", id)
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, title+".epub"), []byte("PK-epub"), 0o644)
		_ = os.WriteFile(filepath.Join(dir, "metadata.opf"), []byte(`<package><metadata><dc:title>`+title+`</dc:title><dc:creator>`+author+`</dc:creator></metadata></package>`), 0o644)
	}
	entry("901", "Dune", "Frank Herbert")
	entry("77", "Emma", "Jane Austen")

	out := readBin(t, admin, true)
	state := map[string]map[string]any{}
	for _, b := range out["books"].([]any) {
		m := b.(map[string]any)
		state[m["id"].(string)] = m
	}
	if state["901"]["in_library"] != true || state["901"]["library_at"] != "900" {
		t.Fatalf("Dune still has Calibre #900: %v", state["901"])
	}
	if state["77"]["in_library"] != false {
		t.Fatalf("Emma's only copy was the one removed, so it is missing: %v", state["77"])
	}
}
