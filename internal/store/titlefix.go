package store

// Title fixes: books whose Calibre title carries track or series numbers
// ("01 - Dune"), which an admin can tidy in Calibre through its Content
// server. NovelCheck itself never writes to the Calibre library.

import (
	"strconv"
	"strings"
)

// TitleFix is a book whose Calibre title could be tidied.
type TitleFix struct {
	ID           int64   `db:"id" json:"id"`
	Title        string  `db:"title" json:"title"` // the tidy title NovelCheck shows
	Author       string  `db:"author" json:"author"`
	CalibreTitle string  `db:"title_fix" json:"calibre_title"`
	Series       string  `db:"series" json:"series"`
	SeriesIndex  float64 `db:"series_index" json:"series_index"`
	CalibreID    int     `db:"calibre_id" json:"calibre_id"` // for an "Open in Calibre-Web" link
}

const inCalibre = `EXISTS (SELECT 1 FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id
	WHERE cb.book_id = b.id AND c.source = 'calibre')`

// TitleFixes lists books in Calibre whose title there could be tidied.
func (s *Store) TitleFixes() ([]TitleFix, error) {
	out := []TitleFix{}
	err := s.DB.Select(&out, `SELECT b.id, b.title, b.author, b.title_fix, b.series, b.series_index,
		(SELECT COALESCE(MIN(CAST(cb.external_id AS INTEGER)), 0) FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id
			WHERE cb.book_id = b.id AND c.source = 'calibre') AS calibre_id
		FROM books b WHERE b.title_fix != '' AND `+inCalibre+` ORDER BY b.author, b.series, b.series_index, b.title`)
	return out, err
}

// CountTitleFixes is how many titles could be tidied in Calibre.
func (s *Store) CountTitleFixes() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM books b WHERE b.title_fix != '' AND `+inCalibre)
	return n
}

// CalibreIDs returns Calibre's book ids for a book (more than one when the
// library has duplicates).
func (s *Store) CalibreIDs(bookID int64) []int {
	var ext []string
	_ = s.DB.Select(&ext, `SELECT DISTINCT cb.external_id FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id
		WHERE cb.book_id = ? AND c.source = 'calibre' ORDER BY cb.external_id`, bookID)
	var ids []int
	for _, e := range ext {
		if id, err := strconv.Atoi(e); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// SavedToCalibre records a title and series an admin saved in Calibre, so
// NovelCheck shows them straight away (the next sync reads the same).
func (s *Store) SavedToCalibre(bookID int64, title, series string, index float64) error {
	title, series = strings.TrimSpace(title), strings.TrimSpace(series)
	if series == "" {
		index = 0
	}
	var author string
	if err := s.DB.Get(&author, `SELECT author FROM books WHERE id = ?`, bookID); err != nil {
		return err
	}
	key := NormKey(title, author) // kept as it was if another book already has the new name
	_, err := s.DB.Exec(`UPDATE books SET title = ?, series = ?, series_index = ?, title_fix = '',
		norm_key = CASE WHEN EXISTS (SELECT 1 FROM books o WHERE o.norm_key = ? AND o.id != books.id)
			THEN norm_key ELSE ? END
		WHERE id = ?`, title, series, index, key, key, bookID)
	return err
}

// ChangedInCalibre reports whether a rated book changed in Calibre since its rating.
func (s *Store) ChangedInCalibre(bookID int64) bool {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM books b WHERE b.id = ? AND `+changedCond, bookID)
	return n > 0
}

// AcceptModified moves the ratings' "last seen change" up to now for books
// whose only change in Calibre was our own title edit, so they aren't
// offered for re-rating.
func (s *Store) AcceptModified(bookIDs []int64) error {
	for _, id := range bookIDs {
		if _, err := s.DB.Exec(`UPDATE books AS b SET rated_modified = `+currentModified+`
			WHERE b.id = ? AND b.rated_modified != ''`, id); err != nil {
			return err
		}
	}
	return nil
}
