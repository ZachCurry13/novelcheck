package store

// Books Calibre could not convert to EPUB. Each keeps its reason, so the owner can see why, try again, or
// say "leave it as it is": then it is not offered for converting any more. A book without an EPUB never
// loses a file to format cleanup either way (that only touches books that have one).

// ConvertFailure is one book that did not convert.
type ConvertFailure struct {
	CalibreID string `db:"calibre_id" json:"calibre_id"`
	Title     string `db:"title" json:"title"`
	Format    string `db:"format" json:"format"`
	Reason    string `db:"reason" json:"reason"`
	AsIs      bool   `db:"as_is" json:"as_is"`
	At        string `db:"at" json:"at"`
}

// NoteConvertFailure records why a book did not convert (keeping an earlier "leave as is").
func (s *Store) NoteConvertFailure(calibreID, title, format, reason string) error {
	if len(reason) > 500 {
		reason = reason[:500] + "…"
	}
	_, err := s.DB.Exec(`INSERT INTO convert_failures (calibre_id, title, format, reason) VALUES (?, ?, ?, ?)
		ON CONFLICT(calibre_id) DO UPDATE SET title = excluded.title, format = excluded.format, reason = excluded.reason, at = CURRENT_TIMESTAMP`,
		calibreID, title, format, reason)
	return err
}

// ForgetConvertFailure clears a book that converted after all.
func (s *Store) ForgetConvertFailure(calibreID string) error {
	_, err := s.DB.Exec(`DELETE FROM convert_failures WHERE calibre_id = ?`, calibreID)
	return err
}

// SetConvertAsIs marks a failed book "leave as is" (or offers it for converting again).
func (s *Store) SetConvertAsIs(calibreID string, asIs bool) error {
	res, err := s.DB.Exec(`UPDATE convert_failures SET as_is = ? WHERE calibre_id = ?`, asIs, calibreID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ConvertFailures lists the books that did not convert, newest first.
func (s *Store) ConvertFailures() []ConvertFailure {
	out := []ConvertFailure{}
	_ = s.DB.Select(&out, `SELECT calibre_id, title, format, reason, as_is, at FROM convert_failures ORDER BY at DESC, title LIMIT 500`)
	return out
}

// leftAsIs is the Calibre ids the owner said not to convert.
func (s *Store) leftAsIs() map[string]bool {
	var ids []string
	_ = s.DB.Select(&ids, `SELECT calibre_id FROM convert_failures WHERE as_is = 1`)
	out := map[string]bool{}
	for _, id := range ids {
		out[id] = true
	}
	return out
}
