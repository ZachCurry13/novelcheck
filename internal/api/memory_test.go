package api_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The memory card: counts of what is remembered, a switch, and purging remembered ratings and the stored covers of
// books that are gone (never the covers of books that still exist).
func TestMemoryCardAndPurge(t *testing.T) {
	covers := t.TempDir()
	var server *api.Server
	srv, st := setupWith(t, func(s *api.Server) { server = s; s.Covers.Dir = covers })
	admin := login(t, srv, "admin", "adminpass1")
	here, _ := st.UpsertBook("Still Here", "Pat Free", "", "")
	gone, _ := st.UpsertBook("Gone", "Pat Free", "", "")
	two := 2
	_ = st.SaveAnalysis(gone, store.Analysis{SpiceLevel: &two, Confidence: "high", Model: "llama3.2"})
	_, _ = st.DB.Exec(`DELETE FROM books WHERE id = ?`, gone) // remembered, and its cover is left behind
	img := image.NewRGBA(image.Rect(0, 0, 40, 60))
	for x := 0; x < 40; x++ {
		for y := 0; y < 60; y++ {
			img.Set(x, y, color.RGBA{uint8(x * 6), uint8(y * 4), 90, 255})
		}
	}
	var jpg bytes.Buffer
	_ = jpeg.Encode(&jpg, img, nil)
	for _, id := range []int64{here, gone} {
		if _, _, ok := server.Covers.Keep("book-"+itoa(id), jpg.Bytes()); !ok {
			t.Fatal("setup: cover not stored")
		}
	}
	_, out := admin.do("GET", "/api/admin/memory", nil, false)
	if out["remember"] != true || out["ratings"] != 1.0 || out["covers_kept"] != 1.0 || out["covers_orphans"] != 1.0 {
		t.Fatalf("the card: %v", out)
	}
	if _, out = admin.do("PUT", "/api/admin/memory", map[string]bool{"remember": false}, true); out["remember"] != false {
		t.Fatalf("switched off: %v", out)
	}
	_, out = admin.do("POST", "/api/admin/memory/purge", map[string]any{"what": "covers"}, true)
	if out["deleted"] != 1.0 {
		t.Fatalf("the gone book's cover is deleted: %v", out)
	}
	if _, err := os.Stat(filepath.Join(covers, "book-"+itoa(here)+".jpg")); err != nil {
		t.Fatal("the cover of a book that still exists is kept")
	}
	_, out = admin.do("POST", "/api/admin/memory/purge", map[string]any{"what": "ratings"}, true)
	if out["forgotten"] != 1.0 {
		t.Fatalf("the remembered rating is forgotten: %v", out)
	}
	if res, _ := admin.do("POST", "/api/admin/memory/purge", map[string]any{"what": "everything"}, true); res.StatusCode != 400 {
		t.Fatalf("unknown action: %d", res.StatusCode)
	}
}
