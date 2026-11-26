package api

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/deepread"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestBoxCount(t *testing.T) {
	for title, want := range map[string]int{
		"The Before Series: Books 1-3 : An Epic Fantasy Adventure": 3,
		"Dragon Blood Collection, Books 1–3":                       3,
		"Winter Cursed Trilogy":                                    3,
		"The Complete 5 Book Set":                                  5,
		"Some Duology":                                             2,
		"Winter Cursed":                                            0,
		"A Box Set of Stories":                                     0,
	} {
		if got := boxCount(title); got != want {
			t.Errorf("%q: %d, want %d", title, got, want)
		}
	}
}

// A box set whose own contents list its books, and whose title says how many, is split by itself.
func TestBoxSplitsItselfFromItsContents(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	dir := t.TempDir()
	p := filepath.Join(dir, "Author", "Box (1)", "box.epub")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	f, _ := os.Create(p)
	zw := zip.NewWriter(f)
	nav := func(n int, title string) string {
		return `<navPoint id="n` + string(rune('0'+n)) + `" playOrder="` + string(rune('0'+n)) + `"><navLabel><text>` + title + `</text></navLabel><content src="c` + string(rune('0'+n)) + `.xhtml"/>` +
			`<navPoint id="c` + string(rune('0'+n)) + `"><navLabel><text>Chapter 1</text></navLabel><content src="c` + string(rune('0'+n)) + `.xhtml#a"/></navPoint></navPoint>`
	}
	for name, body := range map[string]string{
		"META-INF/container.xml": `<container><rootfiles><rootfile full-path="c.opf"/></rootfiles></container>`,
		"c.opf":                  `<package><manifest><item id="a" href="c1.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="a"/></spine></package>`,
		"toc.ncx":                `<ncx><navMap>` + nav(1, "Ice Heart") + nav(2, "Ice Crown") + nav(3, "Ice Throne") + `</navMap></ncx>`,
		"c1.xhtml":               `<html><body>x</body></html>`,
	} {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	_ = f.Close()
	cat, _ := st.EnsureCatalog("Calibre Main", "calibre")
	id, _ := st.UpsertBook("The Ice Series, Books 1-3", "An Author", "", "")
	_ = st.AddCopy(cat, id, p, "epub", "1")
	s := &Server{Store: st, Deep: deepread.New(st, dir)}
	if n := s.boxOnce(context.Background(), 4); n != 1 {
		t.Fatalf("split: %d", n)
	}
	if st.BoxState(id) != "split" {
		t.Fatalf("state: %q", st.BoxState(id))
	}
	for _, title := range []string{"Ice Heart", "Ice Crown", "Ice Throne"} {
		if st.MatchBook(title, "An Author") == 0 {
			t.Fatalf("%q was not made", title)
		}
	}
}
