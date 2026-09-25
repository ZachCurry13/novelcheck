package api_test

import (
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestCoversAndReports(t *testing.T) {
	var lib string
	srv, st := setupWith(t, func(s *api.Server) { lib = s.Cfg.CalibreDir })
	admin := login(t, srv, "admin", "adminpass1")
	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	dir := filepath.Join(lib, "Terry Pratchett", "Mort (4)")
	_ = os.MkdirAll(dir, 0o755)
	f, _ := os.Create(filepath.Join(dir, "cover.jpg"))
	_ = jpeg.Encode(f, image.NewGray(image.Rect(0, 0, 600, 900)), nil)
	f.Close()
	mort, _ := st.UpsertBook("Mort", "Terry Pratchett", "", "")
	_ = st.AddCopy(cat, mort, filepath.Join(dir, "Mort.epub"), "epub", "4")
	_ = st.SaveAnalysis(mort, store.Analysis{Classification: "No Spice"})
	plain, _ := st.UpsertBook("No Cover Here", "Someone", "", "")
	_ = st.AddCopy(cat, plain, "calibre-entry:5", "", "5")

	res, _ := admin.do("GET", "/api/books/"+itoa(mort)+"/cover", nil, false)
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/jpeg" || res.Header.Get("Cache-Control") != "private, max-age=3600" {
		t.Fatalf("cover: %d %v", res.StatusCode, res.Header)
	}
	if res, _ := admin.do("GET", "/api/books/"+itoa(plain)+"/cover", nil, false); res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("placeholder: %d %v", res.StatusCode, res.Header)
	}

	// Anyone can report a wrong cover; the admin sees it once per book.
	admin.do("POST", "/api/admin/users", map[string]string{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	for _, c := range []*client{kid, admin} {
		if res, _ := c.do("POST", "/api/books/"+itoa(mort)+"/cover-report", map[string]string{"note": "That's Eric's cover"}, true); res.StatusCode != 200 {
			t.Fatalf("report: %d", res.StatusCode)
		}
	}
	if _, b := kid.do("GET", "/api/books/"+itoa(mort), nil, false); b["my_cover_report"] != true {
		t.Fatal("the reporter sees it's reported")
	}
	if res, _ := kid.do("GET", "/api/admin/cover-reports", nil, false); res.StatusCode != 403 {
		t.Fatal("only admins review reports")
	}
	_, out := admin.do("GET", "/api/admin/cover-reports", nil, false)
	reports := out["reports"].([]any)
	if len(reports) != 1 || reports[0].(map[string]any)["calibre_id"].(float64) != 4 || st.CountCoverReports() != 1 {
		t.Fatalf("one entry per book: %v", out)
	}
	id := itoa(int64(reports[0].(map[string]any)["id"].(float64)))
	if res, _ := admin.do("POST", "/api/admin/cover-reports/"+id+"/fixed", nil, true); res.StatusCode != 200 || st.CountCoverReports() != 0 {
		t.Fatalf("fixed: %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", "/api/admin/cover-reports/"+id+"/fixed", nil, true); res.StatusCode != 404 {
		t.Fatal("already closed")
	}
}

func TestOpenLibraryCover(t *testing.T) {
	var asked int
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked++
		if r.URL.Path != "/b/isbn/9780141439518-M.jpg" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_ = jpeg.Encode(w, image.NewGray(image.Rect(0, 0, 180, 270)), nil)
	}))
	defer fake.Close()
	srv, st := setupWith(t, func(s *api.Server) { s.Covers.OpenLibraryURL = fake.URL })
	admin := login(t, srv, "admin", "adminpass1")
	looked, _ := st.EnsureCatalog(store.LookedUpCatalog, "custom")
	found, _ := st.UpsertBook("Pride and Prejudice", "Jane Austen", "978-0-14-143951-8", "")
	_ = st.AddCopy(looked, found, "lookup:Pride and Prejudice", "list", "")
	missing, _ := st.UpsertBook("Obscure Pamphlet", "Nobody", "9780000000002", "")
	_ = st.AddCopy(looked, missing, "lookup:Obscure", "list", "")

	for i := 0; i < 2; i++ {
		if res, _ := admin.do("GET", "/api/books/"+itoa(found)+"/cover", nil, false); res.Header.Get("Content-Type") != "image/jpeg" {
			t.Fatalf("open library cover: %v", res.Header)
		}
		if res, _ := admin.do("GET", "/api/books/"+itoa(missing)+"/cover", nil, false); res.Header.Get("Content-Type") != "image/svg+xml" {
			t.Fatalf("placeholder when there's none: %v", res.Header)
		}
	}
	if asked != 2 {
		t.Fatalf("each cover (and each miss) is asked for once: %d", asked)
	}
}
