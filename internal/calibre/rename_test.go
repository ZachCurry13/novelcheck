package calibre_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/zachcurry13/novelcheck/internal/calibre"
	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Series and a numbered title, as Calibre stores them.
const seriesSchema = `
ALTER TABLE books ADD COLUMN series_index REAL NOT NULL DEFAULT 1.0;
CREATE TABLE series (id INTEGER PRIMARY KEY, name TEXT);
CREATE TABLE books_series_link (id INTEGER PRIMARY KEY, book INTEGER, series INTEGER);
INSERT INTO books (id, title, path) VALUES (3, '01 - Guards! Guards!', 'Terry Pratchett/01 - Guards! Guards! (3)');
INSERT INTO books_authors_link VALUES (4, 3, 2);
INSERT INTO data VALUES (4, 3, 'EPUB', '01 - Guards! Guards! - Terry Pratchett');
CREATE TABLE tags (id INTEGER PRIMARY KEY, name TEXT);
CREATE TABLE books_tags_link (id INTEGER PRIMARY KEY, book INTEGER, tag INTEGER);
INSERT INTO tags VALUES (1, 'Fiction'), (2, 'Fantasy'), (3, 'Humor');
INSERT INTO books_tags_link VALUES (1, 3, 1), (2, 3, 2), (3, 3, 3);
`

// TestSyncFollowsRenames: tidying a title in Calibre (which also moves its
// files) keeps the same NovelCheck book, rating and all.
func TestSyncFollowsRenames(t *testing.T) {
	lib := makeLibrary(t)
	cdb, err := sqlx.Open("sqlite", filepath.Join(lib, "metadata.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer cdb.Close()
	if _, err := cdb.Exec(seriesSchema); err != nil {
		t.Fatal(err)
	}
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	if _, err := calibre.Sync(st, lib); err != nil {
		t.Fatal(err)
	}
	find := func() store.Book {
		books, _, _ := st.ListBooks(store.BookFilter{Query: "Guards"}, nil)
		if len(books) != 1 {
			t.Fatalf("want one book, got %+v", books)
		}
		return books[0]
	}
	b := find()
	if b.Title != "Guards! Guards!" || b.SeriesIndex != 1 || b.TitleFix != "01 - Guards! Guards!" ||
		b.Genres != ",fantasy,humor," || b.Kind != "fiction" || b.GenreSource != "calibre" || b.Tags != "Fiction, Fantasy, Humor" {
		t.Fatalf("tidy title from the numbered one: %+v", b)
	}
	one := 1
	_ = st.SaveAnalysis(b.ID, store.Analysis{SpiceLevel: &one, Model: "gpt"})

	// The admin tidies it in Calibre and adds the series; Calibre moves the files.
	if _, err := cdb.Exec(`UPDATE books SET title = 'Guards! Guards!', path = 'Terry Pratchett/Guards! Guards! (3)', series_index = 8 WHERE id = 3;
		INSERT INTO series VALUES (1, 'Discworld'); INSERT INTO books_series_link VALUES (1, 3, 1);
		UPDATE data SET name = 'Guards! Guards! - Terry Pratchett' WHERE book = 3`); err != nil {
		t.Fatal(err)
	}
	if _, err := calibre.Sync(st, lib); err != nil {
		t.Fatal(err)
	}
	after := find()
	if after.ID != b.ID || after.Status != "analyzed" || after.Series != "Discworld" || after.SeriesIndex != 8 || after.TitleFix != "" {
		t.Fatalf("same book, rating kept, series from Calibre: %+v", after)
	}
	copies, _ := st.BookCopies(b.ID)
	if len(copies) != 1 || !strings.HasSuffix(copies[0].Path, "/Guards! Guards! (3)/Guards! Guards! - Terry Pratchett.epub") {
		t.Fatalf("the moved file replaces the old one: %+v", copies)
	}

	// A real rename keeps the book too.
	if _, err := cdb.Exec(`UPDATE books SET title = 'Guards! Guards! (Illustrated)' WHERE id = 3`); err != nil {
		t.Fatal(err)
	}
	if _, err := calibre.Sync(st, lib); err != nil {
		t.Fatal(err)
	}
	if renamed := find(); renamed.ID != b.ID || renamed.Title != "Guards! Guards! (Illustrated)" || renamed.Status != "analyzed" {
		t.Fatalf("renamed in place: %+v", renamed)
	}
}
