package store

import (
	"errors"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/titles"
)

// ErrFromCalibre is returned when a book's title comes from Calibre, which
// would put the old one back at the next sync: it is changed there instead.
var ErrFromCalibre = errors.New("this book's title comes from Calibre: change it there (Edit title in Calibre)")

// RenameBook sets the title and author a parent typed for a book that came
// from Check a book, a list, an event or Discover. The old name stays as an
// alias, so the same book found again by that name joins this one. When
// another book already has the new name, the two become one. It returns the
// book's id (the kept one after joining).
func (s *Store) RenameBook(id int64, title, author string) (int64, error) {
	title, author = strings.TrimSpace(title), strings.TrimSpace(author)
	if title == "" || len(title) > 300 || len(author) > 200 {
		return 0, errors.New("type a title (up to 300 characters) and an author (up to 200)")
	}
	var calibre int
	_ = s.DB.Get(&calibre, `SELECT COUNT(*) FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id
		WHERE cb.book_id = ? AND c.source = 'calibre'`, id)
	if calibre > 0 {
		return 0, ErrFromCalibre
	}
	var oldKey string
	if err := s.DB.Get(&oldKey, `SELECT norm_key FROM books WHERE id = ?`, id); err != nil {
		return 0, ErrNotFound
	}
	t := titles.Parse(titles.WithoutAuthor(title, author))
	if t.Title == "" {
		return 0, errors.New("type a title")
	}
	newKey := s.aliasKey(NormKey(t.Title, author))
	var other int64
	_ = s.DB.Get(&other, `SELECT id FROM books WHERE norm_key = ? AND id != ?`, newKey, id)
	if other != 0 { // already known by that name: one book, with this one's details
		kept, err := s.MergeKeeping(other, id, id)
		if err != nil {
			return 0, err
		}
		id = kept // the joined book takes the new name below, whichever entry survived
	}
	tx, err := s.DB.Beginx()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE books SET norm_key = ?, title = ?, author = ?,
		series = CASE WHEN ? != '' THEN ? ELSE series END,
		series_index = CASE WHEN ? > 0 THEN ? ELSE series_index END, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, newKey, t.Title, author, t.Series, t.Series, t.Index, t.Index, id); err != nil {
		return 0, err
	}
	if oldKey != newKey {
		if _, err := tx.Exec(`INSERT INTO book_aliases (norm_key, book_id) VALUES (?, ?)
			ON CONFLICT (norm_key) DO UPDATE SET book_id = excluded.book_id`, oldKey, id); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}
