package store

// Physical libraries: the family's paper books, added by scanning barcodes
// (or typing a title or ISBN, or a cover photo) into a named library such as
// "Living room shelf". A paper book is an ordinary book with a copy in that
// library (format "paper", no file), so ratings, filters and kids' rules
// work as for e-books.

import (
	"errors"
	"strings"
)

// PaperFormat marks a copy that is a printed book on a shelf.
const PaperFormat = "paper"

// ErrNotShelf is returned for a library that isn't a physical one.
var ErrNotShelf = errors.New("that library isn't a paper-book library")

// CreateShelf makes (or finds) the physical library called name. It is the
// family's (no owner), so every parent can add to it. A library of e-books
// with that name can't become one.
func (s *Store) CreateShelf(name string) (int64, error) {
	var existing struct {
		ID       int64 `db:"id"`
		Physical bool  `db:"physical"`
	}
	err := s.DB.Get(&existing, `SELECT id, physical FROM catalogs WHERE name = ?`, name)
	if err == nil {
		if !existing.Physical {
			return 0, errors.New("a library of e-books already has that name; pick another one")
		}
		return existing.ID, nil
	}
	res, err := s.DB.Exec(`INSERT INTO catalogs (name, source, physical) VALUES (?, 'custom', 1)`, name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// IsShelf reports whether catalog id is a physical library.
func (s *Store) IsShelf(id int64) bool {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM catalogs WHERE id = ? AND physical = 1`, id)
	return n > 0
}

// AddToShelf puts a paper copy of the book in the physical library. It
// reports false when the book was already there.
func (s *Store) AddToShelf(shelfID, bookID int64) (bool, error) {
	if !s.IsShelf(shelfID) {
		return false, ErrNotShelf
	}
	res, err := s.DB.Exec(`INSERT INTO catalog_books (catalog_id, book_id, path, format) VALUES (?, ?, ?, ?)
		ON CONFLICT DO NOTHING`, shelfID, bookID, PaperFormat, PaperFormat)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// PaperOnly reports whether the family has the book only on paper (no file
// to send or download).
func (s *Store) PaperOnly(bookID int64) bool {
	var formats []string
	_ = s.DB.Select(&formats, `SELECT DISTINCT format FROM catalog_books WHERE book_id = ?`, bookID)
	paper := false
	for _, f := range formats {
		switch strings.ToLower(f) {
		case PaperFormat:
			paper = true
		case "", "list":
		default:
			return false
		}
	}
	return paper
}
