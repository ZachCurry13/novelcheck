package store

import (
	"database/sql"
	"errors"
	"strings"
)

// DeepRead is one Deep Scan: the AI reading a whole book.
type DeepRead struct {
	ID          int64  `db:"id" json:"id"`
	BookID      int64  `db:"book_id" json:"book_id"`
	Title       string `db:"title" json:"title"`
	Author      string `db:"author" json:"author"`
	Status      string `db:"status" json:"status"` // requested | queued | reading | done | error | declined | cancelled
	Source      string `db:"source" json:"source"` // admin | request | batch | auto
	RequestedBy string `db:"requested_by" json:"requested_by"`
	Reason      string `db:"reason" json:"reason"`
	ApprovedBy  string `db:"approved_by" json:"approved_by"`
	Words       int    `db:"words" json:"words"`
	PartsTotal  int    `db:"parts_total" json:"parts_total"`
	PartsDone   int    `db:"parts_done" json:"parts_done"`
	EstTokens   int    `db:"est_tokens" json:"est_tokens"`
	Model       string `db:"model" json:"model"`
	Notes       string `db:"notes" json:"notes"` // JSON: [{"label": "Chapter 12", "level": 4, "note": "…"}]
	Error       string `db:"error" json:"error"`
	PrevLevel   *int   `db:"prev_level" json:"prev_level"`
	NewLevel    *int   `db:"new_level" json:"new_level"`
	CreatedAt   string `db:"created_at" json:"created_at"`
	UpdatedAt   string `db:"updated_at" json:"updated_at"`
}

// DeepModelPrefix marks ratings made by a Deep Scan ("deep: <model>"): batch
// re-rates leave them alone, like hand ratings.
const DeepModelPrefix = "deep: "

// Setting keys for Deep Scan.
const (
	KeyDeepModel = "deep_read_model" // optional model for Deep Scans ("" = the main AI's models)
	KeyDeepUsers = "deep_scan_users" // up to 3 user ids whose Up Next is always Deep Scanned
	KeyDeepTopN  = "deep_scan_top_n" // how many Up Next books "Deep Scan next" takes (10, 20, 30)
)

// MaxDeepUsers is how many accounts can have their Up Next scanned automatically.
const MaxDeepUsers = 3

func init() {
	Defaults[KeyDeepModel] = ""
	Defaults[KeyDeepUsers] = ""
	Defaults[KeyDeepTopN] = "10"
}

// ErrDeepReadOpen means the book already has a Deep Scan requested or running.
var ErrDeepReadOpen = errors.New("this book already has a Deep Scan requested or running")

const deepCols = `d.id, d.book_id, b.title, b.author, d.status, d.source, d.requested_by, d.reason, d.approved_by,
	d.words, d.parts_total, d.parts_done, d.est_tokens, d.model, d.notes, d.error, d.prev_level, d.new_level,
	d.created_at, d.updated_at`

// deepChangeCol is "2→4" when the book's latest Deep Scan raised its rating.
const deepChangeCol = `COALESCE((SELECT CASE WHEN dc.new_level > dc.prev_level THEN dc.prev_level || '→' || dc.new_level ELSE '' END
	FROM deep_reads dc WHERE dc.book_id = b.id AND dc.status = 'done' ORDER BY dc.id DESC LIMIT 1), '') AS deep_change`

// RequestDeepRead records a Deep Scan. Approved ones (admin, batch, auto) go
// straight into the queue; requests wait for an admin.
func (s *Store) RequestDeepRead(bookID int64, source, by, reason string, approved bool, words, parts, estTokens int) (int64, error) {
	status, approver := "requested", ""
	if approved {
		status, approver = "queued", by
	}
	res, err := s.DB.Exec(`INSERT INTO deep_reads (book_id, status, source, requested_by, reason, approved_by, words, parts_total, est_tokens)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, bookID, status, source, by, strings.TrimSpace(reason), approver, words, parts, estTokens)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return 0, ErrDeepReadOpen
		}
		return 0, err
	}
	return res.LastInsertId()
}

// LatestDeepRead is the book's most recent Deep Scan, or nil.
func (s *Store) LatestDeepRead(bookID int64) *DeepRead {
	var d DeepRead
	if err := s.DB.Get(&d, `SELECT `+deepCols+` FROM deep_reads d JOIN books b ON b.id = d.book_id
		WHERE d.book_id = ? ORDER BY d.id DESC LIMIT 1`, bookID); err != nil {
		return nil
	}
	return &d
}

// DeepReadByID returns one Deep Scan.
func (s *Store) DeepReadByID(id int64) (*DeepRead, error) {
	var d DeepRead
	err := s.DB.Get(&d, `SELECT `+deepCols+` FROM deep_reads d JOIN books b ON b.id = d.book_id WHERE d.id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &d, err
}

// DeepReads lists open Deep Scans first, then the most recent finished ones
// (the audit log of rating changes).
func (s *Store) DeepReads(limit int) ([]DeepRead, error) {
	out := []DeepRead{}
	err := s.DB.Select(&out, `SELECT `+deepCols+` FROM deep_reads d JOIN books b ON b.id = d.book_id
		ORDER BY d.status NOT IN ('requested', 'queued', 'reading'), d.id DESC LIMIT ?`, limit)
	return out, err
}

// PendingDeepRequests counts requests waiting for an admin.
func (s *Store) PendingDeepRequests() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM deep_reads WHERE status = 'requested'`)
	return n
}

// DecideDeepRead approves or declines a request, or cancels a queued or
// running scan. It reports false if the scan wasn't in a fitting state.
func (s *Store) DecideDeepRead(id int64, action, by string) (bool, error) {
	var q string
	switch action {
	case "approve":
		q = `UPDATE deep_reads SET status = 'queued', approved_by = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status = 'requested'`
	case "decline":
		q = `UPDATE deep_reads SET status = 'declined', approved_by = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status = 'requested'`
	case "cancel":
		q = `UPDATE deep_reads SET status = 'cancelled', approved_by = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status IN ('queued', 'reading')`
	default:
		return false, errors.New("action must be approve, decline or cancel")
	}
	res, err := s.DB.Exec(q, by, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// NextDeepRead is the oldest approved Deep Scan waiting to start.
func (s *Store) NextDeepRead() (*DeepRead, bool) {
	var d DeepRead
	if err := s.DB.Get(&d, `SELECT `+deepCols+` FROM deep_reads d JOIN books b ON b.id = d.book_id
		WHERE d.status = 'queued' ORDER BY d.id LIMIT 1`); err != nil {
		return nil, false
	}
	return &d, true
}

// DeepReadStatus is a scan's current status (to notice a cancel).
func (s *Store) DeepReadStatus(id int64) string {
	var st string
	_ = s.DB.Get(&st, `SELECT status FROM deep_reads WHERE id = ?`, id)
	return st
}

// SetDeepProgress records how far reading has got (never overriding a cancel).
func (s *Store) SetDeepProgress(id int64, done, total int, model string, prevLevel *int) error {
	_, err := s.DB.Exec(`UPDATE deep_reads SET status = 'reading', parts_done = ?, parts_total = ?, model = ?,
		prev_level = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status IN ('queued', 'reading')`,
		done, total, model, prevLevel, id)
	return err
}

// FinishDeepRead stores the result ("done", with the new level) or why it
// failed ("error").
func (s *Store) FinishDeepRead(id int64, status, notes, errMsg string, newLevel *int) error {
	_, err := s.DB.Exec(`UPDATE deep_reads SET status = ?, notes = ?, error = ?, new_level = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status IN ('queued', 'reading')`, status, notes, errMsg, newLevel, id)
	return err
}

// ResumeDeepReads puts scans interrupted by a restart back in the queue.
func (s *Store) ResumeDeepReads() {
	_, _ = s.DB.Exec(`UPDATE deep_reads SET status = 'queued', parts_done = 0 WHERE status = 'reading'`)
}

// DeepCandidates lists books waiting in Up Next (in reading-list order,
// optionally only for some users) that never had a Deep Scan. Ones that
// failed, were declined or cancelled aren't picked again automatically; an
// admin can still start them by hand.
func (s *Store) DeepCandidates(userIDs []int64, limit int) ([]int64, error) {
	q := `SELECT q.book_id FROM queue_items q JOIN books b ON b.id = q.book_id
		WHERE q.status = 'queued' AND b.analysis_model NOT LIKE 'deep:%' AND b.analysis_model NOT LIKE 'manual:%'
		AND NOT EXISTS (SELECT 1 FROM deep_reads d WHERE d.book_id = q.book_id)`
	args := []any{}
	if len(userIDs) > 0 {
		q += ` AND q.user_id IN (?` + strings.Repeat(", ?", len(userIDs)-1) + `)`
		for _, id := range userIDs {
			args = append(args, id)
		}
	}
	q += ` GROUP BY q.book_id ORDER BY MIN(q.position), MIN(q.id) LIMIT ?`
	var ids []int64
	err := s.DB.Select(&ids, q, append(args, limit)...)
	return ids, err
}
