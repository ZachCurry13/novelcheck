package epub

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeEPUB builds a small EPUB with the given files.
func writeEPUB(t *testing.T, files map[string]string) string {
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

func TestReadFollowsSpine(t *testing.T) {
	p := writeEPUB(t, map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
			<rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf": `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0">
			<manifest>
				<item id="c2" href="Text/chapter%202.xhtml" media-type="application/xhtml+xml"/>
				<item id="c1" href="Text/ch1.xhtml" media-type="application/xhtml+xml"/>
				<item id="css" href="style.css" media-type="text/css"/>
				<item id="img" href="cover.jpg" media-type="image/jpeg"/>
			</manifest>
			<spine><itemref idref="c1"/><itemref idref="css"/><itemref idref="c2"/><itemref idref="missing"/></spine></package>`,
		"OEBPS/Text/ch1.xhtml": `<html><head><title>Book</title><style>p{color:red}</style></head>
			<body><h1 class="c">Chapter&nbsp;One</h1><p>It was a &ldquo;dark&rdquo; night.</p><p>They   kissed.</p><script>x()</script></body></html>`,
		"OEBPS/Text/chapter 2.xhtml": `<html><body><h2>Chapter <em>Two</em></h2><div>The end.</div></body></html>`,
		"OEBPS/style.css":            `p{}`,
	})
	secs, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(secs) != 2 || secs[0].Title != "Chapter One" || secs[1].Title != "Chapter Two" {
		t.Fatalf("sections: %+v", secs)
	}
	if secs[0].Text != "Chapter One\nIt was a “dark” night.\nThey kissed." {
		t.Fatalf("text: %q", secs[0].Text)
	}
	if strings.Contains(secs[0].Text, "color") || strings.Contains(secs[0].Text, "x()") || strings.Contains(secs[0].Text, "Book") {
		t.Fatalf("head, style and script must be dropped: %q", secs[0].Text)
	}
	if n := Words(secs); n != 13 { // 9 in chapter one, 4 in chapter two

		t.Fatalf("words: %d", n)
	}
}

func TestReadRejectsNonEPUB(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "x.epub")
	_ = os.WriteFile(bad, []byte("not a zip"), 0o600)
	if _, err := Read(bad); err == nil {
		t.Fatal("a non-zip must fail")
	}
	empty := writeEPUB(t, map[string]string{"mimetype": "application/epub+zip"})
	if _, err := Read(empty); err == nil || !strings.Contains(err.Error(), "container") {
		t.Fatalf("missing container: %v", err)
	}
}
