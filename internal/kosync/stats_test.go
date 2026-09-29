package kosync

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// MakeStats writes a small statistics.sqlite3 like KOReader's (for tests).
func MakeStats(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE book (id integer PRIMARY KEY autoincrement, title text, authors text, notes integer, last_open integer,
			highlights integer, pages integer, series text, language text, md5 text, total_read_time integer, total_read_pages integer)`,
		`CREATE TABLE page_stat_data (id_book integer, page integer NOT NULL DEFAULT 0, start_time integer NOT NULL DEFAULT 0,
			duration integer NOT NULL DEFAULT 0, total_pages integer NOT NULL DEFAULT 0, UNIQUE (id_book, page, start_time))`,
		`INSERT INTO book (title, authors, last_open, pages, md5, total_read_time) VALUES
			('Dune', 'Frank Herbert', 1790000000, 400, 'ABC123', 7200), ('', 'Nobody', 0, 0, '', 0),
			('Mort', 'Terry Pratchett' || char(10) || 'Someone Else', 1780000000, 300, 'def456', 600)`,
		`INSERT INTO page_stat_data VALUES (1, 10, 100, 30, 400), (1, 172, 300, 30, 400), (1, 50, 200, 30, 400), (3, 300, 50, 20, 300)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReadStats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "statistics.sqlite3")
	MakeStats(t, path)
	books, err := ReadStats(path)
	if err != nil || len(books) != 2 {
		t.Fatalf("%+v %v", books, err)
	}
	d, m := books[0], books[1]
	if d.Title != "Dune" || d.MD5 != "abc123" || d.LastPage != 172 || d.Percent != 0.43 || d.ReadSeconds != 7200 || d.LastOpen.Unix() != 1790000000 {
		t.Fatalf("dune: %+v", d)
	}
	if m.Authors != "Terry Pratchett" || m.Percent != 1 {
		t.Fatalf("mort: %+v", m)
	}
	if _, err := ReadStats(filepath.Join(t.TempDir(), "missing.sqlite3")); err == nil {
		t.Fatal("a missing file")
	}
}
