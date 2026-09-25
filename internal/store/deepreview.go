package store

// Deep Scan results an admin reviews: a scan that would raise a book by two
// or more levels waits with its evidence (Accept / Keep the old rating), and
// ratings from scans made before the stricter checks can be found.

import (
	"encoding/json"
	"errors"
)

// DeepChecks is the version of Deep Scan's checks. 2 = romance-only
// peppers, evidence, a second look at 3+ parts, flags backed by more than
// one part (ratings from version 1 could come from one misread part).
const DeepChecks = 2

// HoldDeepRead finishes a scan whose result waits for an admin: the book
// keeps its rating until the proposal is accepted.
func (s *Store) HoldDeepRead(id int64, notes string, a Analysis, level int) error {
	proposal, err := json.Marshal(a)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`UPDATE deep_reads SET status = 'done', notes = ?, error = '', held = 1, proposed_level = ?,
		proposal = ?, checks = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status IN ('queued', 'reading')`,
		notes, level, string(proposal), DeepChecks, id)
	return err
}

// AcceptDeepRead saves a held scan's rating. It reports false when the scan
// isn't waiting any more.
func (s *Store) AcceptDeepRead(id int64, by string) (bool, error) {
	var row struct {
		BookID   int64  `db:"book_id"`
		Proposal string `db:"proposal"`
	}
	if err := s.DB.Get(&row, `SELECT book_id, proposal FROM deep_reads WHERE id = ? AND held = 1`, id); err != nil {
		return false, nil
	}
	var a Analysis
	if err := json.Unmarshal([]byte(row.Proposal), &a); err != nil || a.SpiceLevel == nil {
		return false, errors.New("this Deep Scan's result can't be read; scan the book again")
	}
	if err := s.SaveAnalysis(row.BookID, a); err != nil {
		return false, err
	}
	_, err := s.DB.Exec(`UPDATE deep_reads SET held = 0, new_level = proposed_level, approved_by = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, by, id)
	return err == nil, err
}

// KeepOldRating turns down a held scan: the book keeps the rating it had.
func (s *Store) KeepOldRating(id int64, by string) (bool, error) {
	res, err := s.DB.Exec(`UPDATE deep_reads SET held = 0, status = 'declined', approved_by = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND held = 1`, by, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// HeldDeepReads is how many scans wait for an admin.
func (s *Store) HeldDeepReads() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM deep_reads WHERE held = 1`)
	return n
}

// OldDeepRatings lists books whose rating comes from a Deep Scan made with
// older checks.
func (s *Store) OldDeepRatings() []int64 {
	ids := []int64{}
	_ = s.DB.Select(&ids, `SELECT b.id FROM books b WHERE b.analysis_model LIKE 'deep:%'
		AND NOT EXISTS (SELECT 1 FROM deep_reads d WHERE d.book_id = b.id AND d.status = 'done' AND d.checks >= ?)
		ORDER BY b.id`, DeepChecks)
	return ids
}
