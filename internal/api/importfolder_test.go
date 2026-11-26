package api_test

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The watch folder adds one e-book per book (the best format), unpacks zips, ignores the extras, deletes what it added, moves
// what it could not add to "failed", and leaves what is still being copied alone.
func TestWatchFolder(t *testing.T) {
	cal := &fakeCalibreDetails{book: map[string]any{"title": "Cinder", "authors": "Marissa Meyer", "series": "", "series_index": 1.0,
		"tags": []string{}, "comments": "", "identifiers": map[string]string{}}}
	cs := cal.serve(t)
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": cs.URL}, true)

	dir := t.TempDir()
	old := time.Now().Add(-time.Hour)
	write := func(rel, body string, mod time.Time) {
		p := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		_ = os.Chtimes(p, mod, mod)
	}
	write("Cinder/Cinder.epub", "EPUBDATA", old)
	write("Cinder/Cinder.mobi", "MOBIDATA", old)
	write("Cinder/notes.txt", "nothing", old)
	write("Bad/readme.txt", "no book here", old)
	write("Fresh/New.epub", "COPYING", time.Now())
	for _, d := range []string{"Cinder", "Bad"} {
		_ = os.Chtimes(filepath.Join(dir, d), old, old) // the folder itself counts as changed when something was put in it
	}
	zf, _ := os.Create(filepath.Join(dir, "pack.zip"))
	zw := zip.NewWriter(zf)
	for name, body := range map[string]string{"Scarlet/Scarlet.epub": "SCARLET", "Scarlet/cover-note.txt": "x", "../escape.epub": "BAD"} {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	_ = zf.Close()
	_ = os.Chtimes(filepath.Join(dir, "pack.zip"), old, old)

	if res, out := admin.do("PUT", "/api/admin/import", map[string]any{"enabled": true, "dir": dir, "keep": false}, true); res.StatusCode != 200 || out["dir"] != dir {
		t.Fatalf("settings: %d %v", res.StatusCode, out)
	}
	if res, _ := admin.do("PUT", "/api/admin/import", map[string]any{"enabled": true, "dir": "relative/path"}, true); res.StatusCode != 400 {
		t.Fatalf("a relative folder is refused: %d", res.StatusCode)
	}
	admin.do("POST", "/api/admin/import/now", nil, true)
	var last string
	for i := 0; i < 100 && !strings.Contains(last, "added"); i++ {
		time.Sleep(50 * time.Millisecond)
		_, out := admin.do("GET", "/api/admin/import", nil, false)
		last, _ = out["last"].(string)
	}
	// Cinder is added once; Bad (no book) and the zip (a path that leaves its folder, refused whole) fail
	if !strings.Contains(last, "1 added") || !strings.Contains(last, "2 failed") {
		t.Fatalf("last look: %q", last)
	}
	if len(cal.uploads) != 1 || !strings.Contains(cal.uploads[0], "/cdb/add-book/0/n/Cinder.epub/") {
		t.Fatalf("calibre got: %v", cal.uploads)
	}
	for _, gone := range []string{"Cinder", "pack.zip", "Bad"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); err == nil {
			t.Fatalf("%s should have been taken out of the folder", gone)
		}
	}
	for _, kept := range []string{"Fresh/New.epub", "failed/Bad/readme.txt", "failed/pack.zip"} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Fatalf("%s should be there: %v", kept, err)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape.epub")); err == nil {
		t.Fatal("the zip wrote outside its folder")
	}
}
