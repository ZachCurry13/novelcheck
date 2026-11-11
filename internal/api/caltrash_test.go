package api_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
)

// Calibre's recycle bin is read from the library folder: each removed book with the title and authors it had,
// its files, and when it went in.
func TestCalibreRecycleBinList(t *testing.T) {
	lib := t.TempDir()
	srv, _ := setupWith(t, func(s *api.Server) { s.Cfg.CalibreDir, s.Syncer.Dir = lib, lib })
	admin := login(t, srv, "admin", "adminpass1")

	if _, out := admin.do("GET", "/api/admin/calibre/trash", nil, false); out["found"] != false {
		t.Fatalf("no bin yet: %v", out)
	}
	book := filepath.Join(lib, ".caltrash", "b", "804", "Rant (804)")
	_ = os.MkdirAll(book, 0o755)
	_ = os.WriteFile(filepath.Join(book, "Rant - Chuck Palahniuk.epub"), []byte("PK-epub"), 0o644)
	_ = os.WriteFile(filepath.Join(book, "cover.jpg"), []byte("jpg"), 0o644)
	_ = os.WriteFile(filepath.Join(book, "metadata.opf"), []byte(`<package><metadata><dc:title>Rant: An Oral Biography of Buster Casey</dc:title>
		<dc:creator opf:role="aut">Chuck Palahniuk</dc:creator></metadata></package>`), 0o644)
	_ = os.MkdirAll(filepath.Join(lib, ".caltrash", "b", "1618"), 0o755) // an empty entry

	res, out := admin.do("GET", "/api/admin/calibre/trash", nil, false)
	if res.StatusCode != 200 || out["found"] != true || out["count"].(float64) != 2 {
		t.Fatalf("two books in the bin: %d %v", res.StatusCode, out)
	}
	var rant map[string]any
	for _, b := range out["books"].([]any) {
		if m := b.(map[string]any); m["id"] == "804" {
			rant = m
		}
	}
	files, _ := rant["files"].([]any)
	if rant["title"] != "Rant: An Oral Biography of Buster Casey" || rant["author"] != "Chuck Palahniuk" || len(files) != 1 || files[0].(map[string]any)["format"] != "EPUB" {
		t.Fatalf("the book as it was: %v", rant)
	}
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	if res, _ := kid.do("GET", "/api/admin/calibre/trash", nil, false); res.StatusCode == 200 {
		t.Fatal("not for kids")
	}
}
