package store

import (
	"errors"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/titles"
)

// ErrIsTheName: the Calibre entry's own title is the name the book is known by, so it is not what holds it there.
var ErrIsTheName = errors.New("this entry's title is the book's own name")

// SplitCalibreEntry makes one Calibre entry a book of its own again, after it was joined to another book by name (a
// "same book" join, or two books with the same ISBN): the join that held it goes (the alias of its title's key), and its
// link is dropped, so the next read of Calibre files it under the book its own title names.
func (s *Store) SplitCalibreEntry(extID, title, author string) error {
	t := titles.Parse(titles.WithoutAuthor(title, author))
	if t.Title == "" {
		return errors.New("that entry has no title")
	}
	key := NormKey(t.Title, strings.TrimSpace(author))
	var cur struct {
		ID  int64  `db:"id"`
		Key string `db:"norm_key"`
	}
	if err := s.DB.Get(&cur, `SELECT b.id, b.norm_key FROM books b JOIN catalog_books cb ON cb.book_id = b.id
		JOIN catalogs c ON c.id = cb.catalog_id AND c.source = 'calibre' WHERE cb.external_id = ? LIMIT 1`, extID); err != nil {
		return ErrNotFound
	}
	if key == cur.Key {
		return ErrIsTheName
	}
	res, err := s.DB.Exec(`DELETE FROM book_aliases WHERE norm_key = ? AND book_id = ?`, key, cur.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var other int
		_ = s.DB.Get(&other, `SELECT COUNT(*) FROM books WHERE norm_key = ? AND id != ?`, key, cur.ID)
		if other == 0 {
			return errors.New("NovelCheck keeps that entry with this book for another reason than its name; send me the bug link")
		}
	}
	_, err = s.DB.Exec(`DELETE FROM catalog_books WHERE book_id = ? AND external_id = ? AND catalog_id IN (SELECT id FROM catalogs WHERE source = 'calibre')`, cur.ID, extID)
	return err
}
