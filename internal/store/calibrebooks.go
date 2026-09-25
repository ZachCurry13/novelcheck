package store

// Calibre entries. NovelCheck shows each book's tidy title (see package
// titles) and follows Calibre's own book id, so renaming a book in Calibre
// (or from NovelCheck, through the Content server) keeps its rating, notes
// and queue places instead of turning it into a new, unrated book.

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/titles"
)

// CalibreEntry is one book as Calibre's metadata.db describes it.
type CalibreEntry struct {
	ExtID       string // Calibre's book id
	Title       string
	Authors     string
	ISBN        string
	Description string
	Series      string  // Calibre's series; "" = none
	SeriesIndex float64 // only meaningful with a series
}

// CopyKey identifies one catalog link: a file (or placeholder) of a book.
type CopyKey struct {
	Book int64
	Path string
}

// UpsertCalibreBook adds or updates the book for a Calibre entry and returns its id.
func (s *Store) UpsertCalibreBook(catalogID int64, e CalibreEntry) (int64, error) {
	t := titles.Parse(e.Title)
	if t.Title == "" {
		return 0, errors.New("title required")
	}
	if err := s.followRename(catalogID, e.ExtID, t.Title, e.Authors); err != nil {
		return 0, err
	}
	id, err := s.UpsertBook(t.Title, e.Authors, e.ISBN, e.Description)
	if err != nil {
		return 0, err
	}
	series, index := t.Series, t.Index
	if e.Series != "" { // Calibre's own series wins over what the title suggests
		series, index = e.Series, e.SeriesIndex
	}
	fix := ""
	if t.Title != strings.Join(strings.Fields(e.Title), " ") {
		fix = e.Title
	}
	_, err = s.DB.Exec(`UPDATE books SET series = ?, series_index = ?, title_fix = ? WHERE id = ?`, series, index, fix, id)
	return id, err
}

// followRename keeps a Calibre entry on the same book when its title or
// author changes in Calibre: the book is renamed in place. A new name that
// matches another book, or a book that also has copies elsewhere, is left
// for the normal matching (the entry then joins the book it now matches).
func (s *Store) followRename(catalogID int64, extID, title, author string) error {
	var cur struct {
		ID     int64  `db:"id"`
		Key    string `db:"norm_key"`
		Title  string `db:"title"`
		Author string `db:"author"`
	}
	err := s.DB.Get(&cur, `SELECT b.id, b.norm_key, b.title, b.author FROM books b
		JOIN catalog_books cb ON cb.book_id = b.id WHERE cb.catalog_id = ? AND cb.external_id = ? LIMIT 1`,
		catalogID, extID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // a new entry
	}
	author = strings.TrimSpace(author)
	if err != nil || (cur.Title == title && cur.Author == author) {
		return err
	}
	key := NormKey(title, author)
	if key != cur.Key {
		var others int
		if err := s.DB.Get(&others, `SELECT (SELECT COUNT(*) FROM catalog_books WHERE book_id = ?
			AND NOT (catalog_id = ? AND external_id = ?)) + (SELECT COUNT(*) FROM books WHERE norm_key = ?)`,
			cur.ID, catalogID, extID, key); err != nil || others > 0 {
			return err
		}
	}
	_, err = s.DB.Exec(`UPDATE books SET norm_key = ?, title = ?, author = ? WHERE id = ?`, key, title, author, cur.ID)
	return err
}

// PruneStaleCopies drops a catalog's links that a sync didn't see: files
// Calibre moved or renamed, and entries now filed under another book. Books
// left without any copy are deleted, as in PruneCatalog.
func (s *Store) PruneStaleCopies(catalogID int64, seen map[CopyKey]bool) error {
	var links []struct {
		Book int64  `db:"book_id"`
		Path string `db:"path"`
	}
	if err := s.DB.Select(&links, `SELECT book_id, path FROM catalog_books WHERE catalog_id = ?`, catalogID); err != nil {
		return err
	}
	for _, l := range links {
		if seen[CopyKey{l.Book, l.Path}] {
			continue
		}
		if _, err := s.DB.Exec(`DELETE FROM catalog_books WHERE catalog_id = ? AND book_id = ? AND path = ?`,
			catalogID, l.Book, l.Path); err != nil {
			return err
		}
	}
	_, err := s.DB.Exec(`DELETE FROM books WHERE id NOT IN (SELECT book_id FROM catalog_books)`)
	return err
}

// TidyTitles gives books stored before titles were tidied their plain title
// and series number (run at start-up; books already tidy are untouched).
// A title that would match another book is left as it is.
func (s *Store) TidyTitles() (int, error) {
	var books []struct {
		ID     int64  `db:"id"`
		Key    string `db:"norm_key"`
		Title  string `db:"title"`
		Author string `db:"author"`
	}
	if err := s.DB.Select(&books, `SELECT id, norm_key, title, author FROM books`); err != nil {
		return 0, err
	}
	n := 0
	for _, b := range books {
		t := titles.Parse(b.Title)
		if t.Title == strings.Join(strings.Fields(b.Title), " ") {
			continue
		}
		res, err := s.DB.Exec(`UPDATE books SET norm_key = ?, title = ?, title_fix = ?,
			series = CASE WHEN series = '' THEN ? ELSE series END,
			series_index = CASE WHEN series_index = 0 THEN ? ELSE series_index END
			WHERE id = ? AND NOT EXISTS (SELECT 1 FROM books o WHERE o.norm_key = ? AND o.id != ?)`,
			NormKey(t.Title, b.Author), t.Title, b.Title, t.Series, t.Index, b.ID, NormKey(t.Title, b.Author), b.ID)
		if err != nil {
			return n, err
		}
		if k, _ := res.RowsAffected(); k > 0 {
			n++
		}
	}
	return n, nil
}
