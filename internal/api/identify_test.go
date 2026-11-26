package api

import (
	"archive/zip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/deepread"
	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestIdentifyHelpers(t *testing.T) {
	for title, want := range map[string]bool{"_OceanofPDF.com_My_Merry_Mistake": true, "Unknown": true, "x.epub": true, "Emma": false, "The Gunslinger": false} {
		if looksOff(title) != want {
			t.Errorf("looksOff(%q) = %v", title, !want)
		}
	}
	if !printedIn("... THE GUNSLINGER by Stephen King ...", "The Gunslinger") || printedIn("a story about a cat", "The Gunslinger") {
		t.Error("printedIn: the title must be in the text")
	}
	same := store.Identity{Title: "The Gunslinger", Author: "Stephen King"}
	for _, c := range []struct {
		a, b store.Identity
		want string
	}{
		{same, store.Identity{Title: "Gunslinger, The", Author: "King, Stephen"}, "merge"},
		{same, store.Identity{Title: "The Drawing of the Three", Author: "Stephen King"}, "differ"},
		{store.Identity{Title: "Dune", Author: "Frank Herbert"}, store.Identity{Title: "Dune Messiah", Author: "Frank Herbert"}, ""}, // alike but not the same: left to a person
		{same, store.Identity{}, ""},
	} {
		if got := decidePair(c.a, c.b); got != c.want {
			t.Errorf("%+v vs %+v: %q", c.a, c.b, got)
		}
	}
	if id, err := llm.ParseIdentity(`Sure! {"title": " Emma ", "author": "Jane Austen", "certain": true}`); err != nil || id.Title != "Emma" || !id.Certain {
		t.Errorf("parse: %+v %v", id, err)
	}
}

// A book's first pages are read once, the AI's answer counts only when its title is printed in them, and a made-up title is refused.
func TestIdentifyBookReadsTheFirstPages(t *testing.T) {
	answer := `{"title": "Wuthering Heights", "author": "Emily Bronte", "certain": true}`
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": answer}}}, "usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5}})
	}))
	defer ai.Close()
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	_ = st.SetSetting(store.KeyLLMBaseURL, ai.URL)
	_ = st.SetSetting(store.KeyLLMModel, "m")
	dir := t.TempDir()
	p := filepath.Join(dir, "A", "B (1)", "b.epub")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	f, _ := os.Create(p)
	zw := zip.NewWriter(f)
	for name, body := range map[string]string{
		"META-INF/container.xml": `<container><rootfiles><rootfile full-path="c.opf"/></rootfiles></container>`,
		"c.opf":                  `<package><manifest><item id="a" href="a.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="a"/></spine></package>`,
		"a.xhtml":                `<html><body><h1>Title page</h1><p>WUTHERING HEIGHTS by Emily Bronte. Copyright page. Chapter one begins here.</p></body></html>`,
	} {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	_ = f.Close()
	cat, _ := st.EnsureCatalog("Calibre Main", "calibre")
	id, _ := st.UpsertBook("_OceanofPDF.com_Junk", "Unknown", "", "")
	_ = st.AddCopy(cat, id, p, "epub", "9")
	s := &Server{Store: st, Deep: deepread.New(st, dir)}
	got, ok := s.identifyBook(context.Background(), id)
	if !ok || got.Title != "Wuthering Heights" || got.Author != "Emily Bronte" {
		t.Fatalf("identity: %+v %v", got, ok)
	}
	if again, tried := st.IdentityOf(id); !tried || again.Title != "Wuthering Heights" {
		t.Fatalf("kept: %+v", again)
	}
	if ids := st.IdentifyCandidates(10); len(ids) != 0 {
		t.Fatalf("already read, so not a candidate: %v", ids)
	}
	// a made-up title that is not in the pages is refused
	id2, _ := st.UpsertBook("Unknown", "Unknown", "", "")
	_ = st.AddCopy(cat, id2, p, "epub", "10")
	answer = `{"title": "A Tale Of Two Cities", "author": "Dickens", "certain": true}`
	if _, ok := s.identifyBook(context.Background(), id2); ok {
		t.Fatal("a title that is not printed in the pages must be refused")
	}
}
