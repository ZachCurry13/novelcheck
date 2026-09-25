package api_test

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/deepread"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func tinyEPUB(t *testing.T, path string) {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range map[string]string{
		"META-INF/container.xml": `<container><rootfiles><rootfile full-path="c.opf"/></rootfiles></container>`,
		"c.opf":                  `<package><manifest><item id="a" href="a.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="a"/></spine></package>`,
		"a.xhtml":                "<html><body><h1>Chapter 1</h1><p>" + strings.Repeat("word ", 900) + "</p></body></html>",
	} {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	_ = f.Close()
}

func TestDeepScanWorkflow(t *testing.T) {
	calibreDir := t.TempDir()
	srv, st := setupWith(t, func(s *api.Server) {
		s.Cfg.CalibreDir = calibreDir
		s.Deep = deepread.New(s.Store, calibreDir)
	})
	admin := login(t, srv, "admin", "adminpass1")
	cat, _ := st.EnsureCatalog("Calibre Main", "calibre")
	two := 2
	var books []int64
	for _, title := range []string{"First", "Second", "No Epub"} {
		id, _ := st.UpsertBook(title, "A", "", "")
		_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &two, Model: "gpt"})
		if title != "No Epub" {
			p := filepath.Join(calibreDir, title, "book.epub")
			tinyEPUB(t, p)
			_ = st.AddCopy(cat, id, p, "epub", title)
		}
		_ = st.Enqueue(1, id)
		books = append(books, id)
	}

	// What a scan would cost; no EPUB, no scan.
	_, info := admin.do("GET", "/api/books/"+itoa(books[0])+"/deep-scan", nil, false)
	if info["available"] != true || info["estimate"].(map[string]any)["words"].(float64) != 902 {
		t.Fatalf("estimate: %v", info)
	}
	if _, info = admin.do("GET", "/api/books/"+itoa(books[2])+"/deep-scan", nil, false); info["available"] != false || !strings.Contains(info["why"].(string), "EPUB") {
		t.Fatalf("no EPUB: %v", info)
	}

	// An editor asks; the admin approves.
	admin.do("POST", "/api/admin/users", map[string]string{"username": "mom", "password": "editorpass1", "role": "editor"}, true)
	mom := login(t, srv, "mom", "editorpass1")
	if res, out := mom.do("POST", "/api/books/"+itoa(books[0])+"/deep-scan", map[string]string{"reason": "check the middle"}, true); res.StatusCode != 200 || out["queued"] != false {
		t.Fatalf("request: %d %v", res.StatusCode, out)
	}
	if res, _ := mom.do("POST", "/api/books/"+itoa(books[0])+"/deep-scan", nil, true); res.StatusCode != 409 {
		t.Fatalf("one open scan per book: %d", res.StatusCode)
	}
	if _, s := admin.do("GET", "/api/admin/status", nil, false); s["pending_deep"].(float64) != 1 {
		t.Fatalf("pending requests: %v", s["pending_deep"])
	}
	_, list := admin.do("GET", "/api/admin/deep-scans", nil, false)
	scan := list["scans"].([]any)[0].(map[string]any)
	if scan["status"] != "requested" || scan["source"] != "request" || scan["reason"] != "check the middle" {
		t.Fatalf("scan listing: %v", scan)
	}
	if res, _ := mom.do("POST", "/api/admin/deep-scans/1/approve", nil, true); res.StatusCode != 403 {
		t.Fatalf("only admins approve: %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", "/api/admin/deep-scans/1/approve", nil, true); res.StatusCode != 200 {
		t.Fatalf("approve: %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", "/api/admin/deep-scans/1/approve", nil, true); res.StatusCode != 409 {
		t.Fatalf("approving twice: %d", res.StatusCode)
	}

	// Next Up Next books: only the one that has an EPUB and no scan yet.
	_, next := admin.do("GET", "/api/admin/deep-scans/next?n=10", nil, false)
	if next["books"].(float64) != 1 || next["titles"].([]any)[0] != "Second" {
		t.Fatalf("next estimate: %v", next)
	}
	if _, next = admin.do("POST", "/api/admin/deep-scans/next?n=10", nil, true); next["queued"].(float64) != 1 {
		t.Fatalf("queue next: %v", next)
	}

	// Up to 3 real accounts can be scanned automatically.
	admin.do("PUT", "/api/admin/settings", map[string]string{"deep_scan_users": "1, 99, 2, x"}, true)
	if got := st.Setting(store.KeyDeepUsers); got != "1,2" {
		t.Fatalf("deep scan users: %q", got)
	}

	// A finished scan that raised the rating: the Library filter and the queue banner.
	four := 4
	_ = st.SaveAnalysis(books[1], store.Analysis{SpiceLevel: &four, Model: "deep: m"})
	d, _ := st.NextDeepRead()
	_ = st.SetDeepProgress(d.ID, 1, 1, "m", &two)
	_ = st.FinishDeepRead(d.ID, "done", "[]", "", &four)
	if _, out := admin.do("GET", "/api/books?deep=1", nil, false); out["total"].(float64) != 1 {
		t.Fatalf("deep filter: %v", out["total"])
	}
	items, _ := st.ListQueue(1, false)
	changes := map[string]string{}
	for _, it := range items {
		changes[it.Title] = it.DeepChange
	}
	if changes[d.Title] != "2→4" {
		t.Fatalf("queue banner for %q: %v", d.Title, changes)
	}
}
