package store

// Cover reports: someone says a book's cover is wrong; an admin fixes it in
// Calibre (NovelCheck picks the new cover up by itself) and closes the report.

import "strings"

// CoverReport is an open report, with the book's details for the review list.
type CoverReport struct {
	ID        int64  `db:"id" json:"id"`
	BookID    int64  `db:"book_id" json:"book_id"`
	Title     string `db:"title" json:"title"`
	Author    string `db:"author" json:"author"`
	CalibreID int    `db:"calibre_id" json:"calibre_id"` // for an "Open in Calibre-Web" link; 0 = none
	Username  string `db:"username" json:"username"`
	Note      string `db:"note" json:"note"`
	CreatedAt string `db:"created_at" json:"created_at"`
}

// ReportCover records that u thinks the book's cover is wrong (one open
// report per person and book; reporting again updates the note).
func (s *Store) ReportCover(bookID int64, u *User, note string) error {
	note = strings.TrimSpace(note)
	if r := []rune(note); len(r) > 300 {
		note = string(r[:300])
	}
	_, err := s.DB.Exec(`INSERT INTO cover_reports (book_id, user_id, username, note) VALUES (?, ?, ?, ?)
		ON CONFLICT (book_id, user_id) WHERE status = 'open' DO UPDATE SET note = excluded.note`,
		bookID, u.ID, u.Username, note)
	return err
}

// MyCoverReport reports whether u has an open report on the book.
func (s *Store) MyCoverReport(bookID, userID int64) bool {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM cover_reports WHERE book_id = ? AND user_id = ? AND status = 'open'`, bookID, userID)
	return n > 0
}

// CoverReports lists the books with open reports, oldest first (several
// people reporting one book make one entry).
func (s *Store) CoverReports() ([]CoverReport, error) {
	out := []CoverReport{}
	err := s.DB.Select(&out, `SELECT MIN(r.id) AS id, r.book_id, b.title, b.author, GROUP_CONCAT(r.username, ', ') AS username,
		COALESCE(GROUP_CONCAT(NULLIF(r.note, ''), ' · '), '') AS note, MIN(r.created_at) AS created_at,
		(SELECT COALESCE(MIN(CAST(cb.external_id AS INTEGER)), 0) FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id
			WHERE cb.book_id = b.id AND c.source = 'calibre') AS calibre_id
		FROM cover_reports r JOIN books b ON b.id = r.book_id WHERE r.status = 'open'
		GROUP BY r.book_id ORDER BY MIN(r.created_at), MIN(r.id)`)
	return out, err
}

// CountCoverReports is how many reports are waiting.
func (s *Store) CountCoverReports() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(DISTINCT book_id) FROM cover_reports WHERE status = 'open'`)
	return n
}

// CloseCoverReports marks every open report on the report's book fixed or
// dismissed. It reports false when the report isn't open any more.
func (s *Store) CloseCoverReports(id int64, status, by string) (bool, error) {
	res, err := s.DB.Exec(`UPDATE cover_reports SET status = ?, decided_by = ?
		WHERE status = 'open' AND book_id = (SELECT book_id FROM cover_reports WHERE id = ?)`, status, by, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
