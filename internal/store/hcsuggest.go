package store

import "strings"

// HCSuggestion is one correction to a Calibre book suggested from Hardcover.
type HCSuggestion struct {
	ID         int64  `db:"id" json:"id"`
	BookID     int64  `db:"book_id" json:"book_id"`
	CalibreID  int    `db:"calibre_id" json:"calibre_id"`
	Title      string `db:"title" json:"title"`
	Field      string `db:"field" json:"field"`     // title, authors, series, series_index, identifiers, comments, tags
	Current    string `db:"current" json:"current"` // as Calibre has it now, for showing
	Shown      string `db:"shown" json:"shown"`     // what it would become, for showing
	Payload    string `db:"payload" json:"-"`       // the value to write (JSON)
	Confidence string `db:"confidence" json:"confidence"`
	Status     string `db:"status" json:"status"`
}

// CalibreBookRef is a library book with its Calibre number.
type CalibreBookRef struct {
	BookID    int64  `db:"book_id"`
	CalibreID int    `db:"calibre_id"`
	Title     string `db:"title"`
	Author    string `db:"author"`
	ISBN      string `db:"isbn"`
}

// CalibreBookRefs lists every book that is in Calibre, one row per Calibre number.
func (s *Store) CalibreBookRefs() []CalibreBookRef {
	out := []CalibreBookRef{}
	_ = s.DB.Select(&out, `SELECT DISTINCT b.id AS book_id, CAST(cb.external_id AS INTEGER) AS calibre_id, b.title, b.author, b.isbn
		FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id JOIN books b ON b.id = cb.book_id
		WHERE c.source = 'calibre' AND cb.external_id GLOB '[0-9]*' ORDER BY calibre_id`)
	return out
}

// ClearNewHCSuggestions drops the suggestions not yet decided (a new scan replaces them).
func (s *Store) ClearNewHCSuggestions() error {
	_, err := s.DB.Exec(`DELETE FROM hc_suggestions WHERE status = 'new'`)
	return err
}

// AddHCSuggestion keeps a suggestion unless the same change was already applied or skipped.
func (s *Store) AddHCSuggestion(x HCSuggestion) {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM hc_suggestions WHERE calibre_id = ? AND field = ? AND payload = ? AND status IN ('applied', 'skipped')`,
		x.CalibreID, x.Field, x.Payload)
	if n > 0 {
		return
	}
	_, _ = s.DB.Exec(`INSERT INTO hc_suggestions (book_id, calibre_id, title, field, current, shown, payload, confidence)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, x.BookID, x.CalibreID, x.Title, x.Field, x.Current, x.Shown, x.Payload, x.Confidence)
}

// HCSuggestions lists suggestions with the status ("new" by default), grouped by book.
func (s *Store) HCSuggestions(status string, limit int) []HCSuggestion {
	if status == "" {
		status = "new"
	}
	out := []HCSuggestion{}
	_ = s.DB.Select(&out, `SELECT id, book_id, calibre_id, title, field, current, shown, payload, confidence, status FROM hc_suggestions
		WHERE status = ? ORDER BY title COLLATE NOCASE, calibre_id, id LIMIT ?`, status, limit)
	return out
}

// HCSuggestionsByID returns the named suggestions that are still new.
func (s *Store) HCSuggestionsByID(ids []int64) []HCSuggestion {
	out := []HCSuggestion{}
	if len(ids) == 0 {
		return out
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	_ = s.DB.Select(&out, `SELECT id, book_id, calibre_id, title, field, current, shown, payload, confidence, status FROM hc_suggestions
		WHERE status = 'new' AND id IN (`+marks+`) ORDER BY calibre_id, id`, args...)
	return out
}

// SetHCSuggestionStatus marks suggestions applied or skipped.
func (s *Store) SetHCSuggestionStatus(ids []int64, status string) {
	for _, id := range ids {
		_, _ = s.DB.Exec(`UPDATE hc_suggestions SET status = ? WHERE id = ?`, status, id)
	}
}

// CountHCSuggestions counts the suggestions still new.
func (s *Store) CountHCSuggestions() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM hc_suggestions WHERE status = 'new'`)
	return n
}
