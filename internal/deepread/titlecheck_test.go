package deepread

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestTitleFits(t *testing.T) {
	for _, c := range []struct {
		library, file string
		want          bool
	}{
		{"The Gunslinger", "The Dark Tower I: The Gunslinger", true},
		{"Dune", "Dune (Dune Chronicles #1)", true},
		{"Do Androids Dream of Electric Sheep?", "Blade Runner", false}, // another title for the same book: a person decides
		{"Harry Potter and the Sorcerer's Stone", "Harry Potter and the Philosopher's Stone", true},
		{"Emma", "Persuasion", false},
		{"Emma", "Unknown", true},
		{"Emma", "", true},
		{"Quick Start Guide", "Wuthering Heights", false},
	} {
		if got := titleFits(c.library, c.file); got != c.want {
			t.Errorf("%q vs %q: %v", c.library, c.file, got)
		}
	}
}

func TestWrongFile(t *testing.T) {
	for _, c := range []struct {
		lt, la, ft, fa string
		stop, warn     bool
	}{
		{"Emma", "Jane Austen", "Persuasion", "Jane Austen", false, true},         // same author, other title: warn
		{"Emma", "Jane Austen", "Wuthering Heights", "Emily Bronte", true, false}, // another book and author: stop
		{"The Gunslinger", "Stephen King", "The Dark Tower I: The Gunslinger", "King, Stephen", false, false},
		{"Emma", "Jane Austen", "Persuasion", "", false, false}, // no author in the file, title differs: it fits only if the title does
	} {
		stop, warn := wrongFile(c.lt, c.la, c.ft, c.fa)
		if c.fa == "" && c.ft == "Persuasion" { // a missing author is not evidence of another author
			if stop {
				t.Errorf("stopped on a missing author")
			}
			continue
		}
		if stop != c.stop || warn != c.warn {
			t.Errorf("%q/%q vs %q/%q: stop %v warn %v", c.lt, c.la, c.ft, c.fa, stop, warn)
		}
	}
}

// A scan whose file is another book (title and author both differ) stops before reading, says so, and rates nothing.
func TestScanStopsOnTheWrongFile(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	calibreDir := t.TempDir()
	p := filepath.Join(calibreDir, "Author", "Wrong (1)", "wrong.epub")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	f, _ := os.Create(p)
	zw := zip.NewWriter(f)
	for name, body := range map[string]string{
		"META-INF/container.xml": `<container><rootfiles><rootfile full-path="c.opf"/></rootfiles></container>`,
		"c.opf": `<package xmlns:dc="http://purl.org/dc/elements/1.1/"><metadata><dc:title>Wuthering Heights</dc:title><dc:creator>Emily Bronte</dc:creator></metadata>
			<manifest><item id="a" href="a.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="a"/></spine></package>`,
		"a.xhtml": "<html><body><h1>One</h1><p>" + words(500, "moor") + "</p></body></html>",
	} {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	_ = f.Close()
	cat, _ := st.EnsureCatalog("Calibre Main", "calibre")
	id, _ := st.UpsertBook("Quick Start Guide", "John Schember", "", "")
	_ = st.AddCopy(cat, id, p, "epub", "7")
	r := New(st, calibreDir)
	if n, _ := r.Queue([]int64{id}, "admin", "admin"); n != 1 {
		t.Fatal("not queued")
	}
	next, _ := st.NextDeepRead()
	err = r.scan(context.Background(), next)
	if err == nil || !strings.Contains(err.Error(), "wrong file") || !strings.Contains(err.Error(), "Wuthering Heights") {
		t.Fatalf("the scan should stop: %v", err)
	}
	items, _, _ := st.Notifications(5)
	if len(items) == 0 || items[0].Source != "deep-scan-wrong-file" {
		t.Fatalf("notice: %+v", items)
	}
}
