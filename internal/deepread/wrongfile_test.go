package deepread

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// wrongFileBook is a Calibre book "Fourth Wing" whose file says it is "Dune" by Frank Herbert.
func wrongFileBook(t *testing.T) (*Runner, *store.Store, int64, int64) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	st := store.New(d)
	ai := fakeAI(t, true)
	_ = st.SetSetting(store.KeyLLMBaseURL, ai.URL)
	_ = st.SetSetting(store.KeyLLMModel, "m")
	dir := t.TempDir()
	p := filepath.Join(dir, "A", "Fourth Wing (1)", "book.epub")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	f, _ := os.Create(p)
	zw := zip.NewWriter(f)
	for name, body := range map[string]string{
		"META-INF/container.xml": `<container><rootfiles><rootfile full-path="content.opf"/></rootfiles></container>`,
		"content.opf": `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Dune</dc:title><dc:creator>Frank Herbert</dc:creator></metadata>
			<manifest><item id="a" href="a.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="a"/></spine></package>`,
		"a.xhtml": "<html><body><h1>Chapter 1</h1><p>" + words(3000, "sweet") + "</p></body></html>",
	} {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	_ = f.Close()
	cat, _ := st.EnsureCatalog("Calibre Main", "calibre")
	id, _ := st.UpsertBook("Fourth Wing", "Rebecca Yarros", "", "")
	_ = st.AddCopy(cat, id, p, "epub", "1")
	two := 2
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &two, Model: "gpt"})
	r := New(st, dir)
	_ = st.SetSetting(store.KeyLocalContext, "32768")
	if n, _ := r.Queue([]int64{id}, "admin", "admin"); n != 1 {
		t.Fatal("not queued")
	}
	next, _ := st.NextDeepRead()
	return r, st, id, next.ID
}

// A file that is another book is renamed from its own pages and the scan goes on; when the pages settle nothing, it stops as before.
func TestWrongFileIsFixedOrStops(t *testing.T) {
	r, st, id, _ := wrongFileBook(t)
	next, _ := st.NextDeepRead()
	if err := r.scan(context.Background(), next); err == nil {
		t.Fatal("no hook: the scan stops")
	}
	r2, st2, id2, _ := wrongFileBook(t)
	r2.OnWrongFile = func(_ context.Context, b *store.Book) (string, string, FileVerdict) { return "Dune", "Frank Herbert", FileIs }
	next2, _ := st2.NextDeepRead()
	if err := r2.scan(context.Background(), next2); err != nil {
		t.Fatalf("renamed, the scan goes on: %v", err)
	}
	items, _, _ := st2.Notifications(5)
	found := false
	for _, n := range items {
		found = found || n.Source == "deep-scan-fixed"
	}
	if !found {
		t.Fatalf("no notice: %+v", items)
	}
	r3, st3, _, _ := wrongFileBook(t)
	r3.OnWrongFile = func(_ context.Context, b *store.Book) (string, string, FileVerdict) { return "", "", FileAgrees }
	next3, _ := st3.NextDeepRead()
	if err := r3.scan(context.Background(), next3); err != nil {
		t.Fatalf("the pages agree with the library: %v", err)
	}
	_, _, _ = st, id, id2
}
