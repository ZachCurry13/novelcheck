package epub

import (
	"archive/zip"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeTOCBook(t *testing.T, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "book.epub")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	_ = f.Close()
	return p
}

// A box set's contents list its books with their chapters under them; front matter and a flat list of
// chapters are left out.
func TestBooksFromContents(t *testing.T) {
	ncx := `<?xml version="1.0"?><ncx xmlns="http://www.daisy.org/z3986/2005/ncx/"><navMap>
	<navPoint><navLabel><text>Copyright</text></navLabel></navPoint>
	<navPoint><navLabel><text>Book 1: Winter's Curse</text></navLabel>
	  <navPoint><navLabel><text>Chapter 1</text></navLabel></navPoint><navPoint><navLabel><text>Chapter 2</text></navLabel></navPoint></navPoint>
	<navPoint><navLabel><text>Spring&#8217;s Thaw</text></navLabel>
	  <navPoint><navLabel><text>Chapter 1</text></navLabel></navPoint></navPoint>
	<navPoint><navLabel><text>About the Author</text></navLabel><navPoint><navLabel><text>Links</text></navLabel></navPoint></navPoint>
	</navMap></ncx>`
	got, err := Books(writeTOCBook(t, map[string]string{"OEBPS/toc.ncx": ncx}))
	if err != nil || !reflect.DeepEqual(got, []string{"Winter's Curse", "Spring’s Thaw"}) {
		t.Fatalf("books from the ncx: %q %v", got, err)
	}

	flat := `<ncx><navMap><navPoint><navLabel><text>Chapter 1</text></navLabel></navPoint><navPoint><navLabel><text>Chapter 2</text></navLabel></navPoint></navMap></ncx>`
	if got, _ := Books(writeTOCBook(t, map[string]string{"toc.ncx": flat})); got != nil {
		t.Fatalf("a single book's chapters are not books: %q", got)
	}

	nav := `<html xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol>
	<li><a href="t.xhtml">Title Page</a></li>
	<li><a href="a.xhtml">Volume One - Frost</a><ol><li><a href="1.xhtml">One</a></li><li><a href="2.xhtml">Two</a></li></ol></li>
	<li><a href="b.xhtml">Thaw &amp; Bloom</a><ol><li><a href="3.xhtml">One</a></li></ol></li>
	</ol></nav></body></html>`
	got, err = Books(writeTOCBook(t, map[string]string{"OEBPS/nav.xhtml": nav}))
	if err != nil || !reflect.DeepEqual(got, []string{"Frost", "Thaw & Bloom"}) {
		t.Fatalf("books from the nav: %q %v", got, err)
	}
}
