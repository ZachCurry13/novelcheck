package store

// A second look at Deep Scans an admin turned down or set lower than the scan found, when the scan found Level 1
// or 2. Until v1.73.1 the review page listed only the parts rated Level 3 or more, so a Level 1 or 2 suggestion
// showed "0 parts" and looked like a mistake, and some were kept at Level 0 for that reason.

// KeptBelow is one such book.
type KeptBelow struct {
	ID       int64  `db:"id" json:"id"` // the scan
	BookID   int64  `db:"book_id" json:"book_id"`
	Title    string `db:"title" json:"title"`
	Author   string `db:"author" json:"author"`
	Found    int    `db:"proposed_level" json:"found"` // what the scan found
	Now      *int   `db:"spice_level" json:"now"`      // the book's level now
	Decision string `db:"status" json:"decision"`      // declined (kept the old rating) or done (a level was set)
}

// keptBelowCond picks each book's latest scan that found Level 1 or 2 and was decided lower by a person, while
// the book is still rated below what the scan found.
const keptBelowCond = `d.id = (SELECT MAX(x.id) FROM deep_reads x WHERE x.book_id = d.book_id)
	AND d.held = 0 AND d.approved_by != '' AND d.proposal != '' AND d.proposed_level BETWEEN 1 AND 2
	AND (d.status = 'declined' OR (d.status = 'done' AND d.new_level IS NOT NULL AND d.new_level < d.proposed_level))
	AND COALESCE(b.spice_level, 0) < d.proposed_level`

// KeptBelowScan lists those books.
func (s *Store) KeptBelowScan() []KeptBelow {
	out := []KeptBelow{}
	_ = s.DB.Select(&out, `SELECT d.id, d.book_id, b.title, b.author, d.proposed_level, b.spice_level, d.status
		FROM deep_reads d JOIN books b ON b.id = d.book_id WHERE `+keptBelowCond+` ORDER BY b.title`)
	return out
}

// ReopenKeptBelow puts those scans back in the review list (as waiting for a decision), and says how many.
func (s *Store) ReopenKeptBelow() (int, error) {
	res, err := s.DB.Exec(`UPDATE deep_reads SET held = 1, status = 'done', approved_by = '', updated_at = CURRENT_TIMESTAMP
		WHERE id IN (SELECT d.id FROM deep_reads d JOIN books b ON b.id = d.book_id WHERE ` + keptBelowCond + `)`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
