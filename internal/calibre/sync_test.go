package calibre_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/zachcurry13/novelcheck/internal/calibre"
	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// calibreSchema is the subset of Calibre's metadata.db that Sync reads.
const calibreSchema = `
CREATE TABLE books (id INTEGER PRIMARY KEY, title TEXT, path TEXT, isbn TEXT DEFAULT '');
CREATE TABLE authors (id INTEGER PRIMARY KEY, name TEXT);
CREATE TABLE books_authors_link (id INTEGER PRIMARY KEY, book INTEGER, author INTEGER);
CREATE TABLE identifiers (id INTEGER PRIMARY KEY, book INTEGER, type TEXT, val TEXT);
CREATE TABLE comments (id INTEGER PRIMARY KEY, book INTEGER, text TEXT);
CREATE TABLE data (id INTEGER PRIMARY KEY, book INTEGER, format TEXT, name TEXT);
INSERT INTO books VALUES (1, 'Pride and Prejudice', 'Jane Austen/Pride and Prejudice (1)', '');
INSERT INTO books VALUES (2, 'Good Omens', 'Terry Pratchett/Good Omens (2)', '9780060853983');
INSERT INTO authors VALUES (1, 'Jane Austen'), (2, 'Terry Pratchett'), (3, 'Neil Gaiman');
INSERT INTO books_authors_link VALUES (1, 1, 1), (2, 2, 2), (3, 2, 3);
INSERT INTO identifiers VALUES (1, 1, 'isbn', '9780141439518');
INSERT INTO comments VALUES (1, 1, '<p>A <b>classic</b> &amp; witty novel.</p>');
INSERT INTO data VALUES (1, 1, 'EPUB', 'Pride and Prejudice - Jane Austen'), (2, 1, 'MOBI', 'Pride and Prejudice - Jane Austen'),
	(3, 2, 'EPUB', 'Good Omens - Terry Pratchett');
`

func makeLibrary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	c, err := sqlx.Open("sqlite", filepath.Join(dir, "metadata.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(calibreSchema); err != nil {
		t.Fatal(err)
	}
	c.Close()
	return dir
}

func TestSyncReadOnly(t *testing.T) {
	lib := makeLibrary(t)
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)

	before, _ := os.Stat(filepath.Join(lib, "metadata.db"))
	res, err := calibre.Sync(st, lib)
	if err != nil {
		t.Fatal(err)
	}
	if res.Books != 2 || res.Copies != 3 {
		t.Fatalf("unexpected result %+v", res)
	}
	after, _ := os.Stat(filepath.Join(lib, "metadata.db"))
	if !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size() {
		t.Fatal("metadata.db was modified by sync")
	}

	books, _, _ := st.ListBooks(store.BookFilter{Query: "Good Omens"}, nil)
	if len(books) != 1 || books[0].Author != "Terry Pratchett & Neil Gaiman" || books[0].Status != "pending" {
		t.Fatalf("unexpected book %+v", books)
	}
	pp, _, _ := st.ListBooks(store.BookFilter{Query: "Pride"}, nil)
	if pp[0].ISBN != "9780141439518" || pp[0].Description != "A classic & witty novel." {
		t.Fatalf("metadata not extracted: %+v", pp[0])
	}
	copies, _ := st.BookCopies(pp[0].ID)
	want := filepath.Join(lib, "Jane Austen/Pride and Prejudice (1)/Pride and Prejudice - Jane Austen.epub")
	found := false
	for _, c := range copies {
		if c.Path == want && c.CatalogName == store.CalibreCatalogName {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected copy at %s, got %+v", want, copies)
	}

	// Re-sync is idempotent; deleting a book in Calibre prunes it.
	c, _ := sqlx.Open("sqlite", filepath.Join(lib, "metadata.db"))
	c.MustExec(`DELETE FROM books WHERE id = 2`)
	c.Close()
	res, err = calibre.Sync(st, lib)
	if err != nil || res.Books != 1 || res.Removed != 1 {
		t.Fatalf("resync: %+v %v", res, err)
	}
	if b, _, _ := st.ListBooks(store.BookFilter{}, nil); len(b) != 1 {
		t.Fatalf("expected 1 book after prune, got %d", len(b))
	}
}

func TestFilePathUsesMountPrefix(t *testing.T) {
	got := calibre.FilePath("/calibre", "A/B (3)", "B - A", "AZW3")
	if got != "/calibre/A/B (3)/B - A.azw3" {
		t.Fatal(got)
	}
}
