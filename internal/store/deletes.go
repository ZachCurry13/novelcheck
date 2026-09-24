package store

import (
	"errors"
	"strings"

	"github.com/jmoiron/sqlx"
)

// DeleteRequest is one person's request to delete a book.
type DeleteRequest struct {
	ID        int64   `db:"id" json:"id"`
	BookID    *int64  `db:"book_id" json:"book_id"`
	Title     string  `db:"title" json:"title"`
	Author    string  `db:"author" json:"author"`
	Username  string  `db:"username" json:"username"`
	Reason    string  `db:"reason" json:"reason"`
	Status    string  `db:"status" json:"status"` // pending | deleted | dismissed | done
	DecidedBy string  `db:"decided_by" json:"decided_by"`
	CreatedAt string  `db:"created_at" json:"created_at"`
	DecidedAt *string `db:"decided_at" json:"decided_at"`
}

// DeleteGroup is a book with its pending requests, for the admin's review.
type DeleteGroup struct {
	BookID     int64           `json:"book_id"`
	Title      string          `json:"title"`
	Author     string          `json:"author"`
	Formats    string          `json:"formats"`
	CalibreIDs []string        `json:"calibre_ids"`
	Catalogs   string          `json:"catalogs"`
	Requests   []DeleteRequest `json:"requests"`
}

const MaxReasonLen = 500

var DeleteStatuses = map[string]bool{"deleted": true, "dismissed": true, "done": true}

// RequestDelete records (or updates) a user's pending request for a book.
func (s *Store) RequestDelete(b *Book, u *User, reason string) error {
	reason = strings.TrimSpace(reason)
	if len(reason) > MaxReasonLen {
		return errors.New("the reason is too long (500 characters max)")
	}
	_, err := s.DB.Exec(`INSERT INTO delete_requests (book_id, title, author, user_id, username, reason)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (book_id, user_id) WHERE status = 'pending' DO UPDATE SET reason = excluded.reason`,
		b.ID, b.Title, b.Author, u.ID, u.Username, reason)
	return err
}

// CancelDelete withdraws a user's pending request.
func (s *Store) CancelDelete(bookID, userID int64) error {
	_, err := s.DB.Exec(`DELETE FROM delete_requests WHERE book_id = ? AND user_id = ? AND status = 'pending'`, bookID, userID)
	return err
}

// MyDeleteRequest returns the user's pending request for a book, if any.
func (s *Store) MyDeleteRequest(bookID, userID int64) *DeleteRequest {
	var r DeleteRequest
	if err := s.DB.Get(&r, `SELECT id, book_id, title, author, username, reason, status, decided_by, created_at, decided_at
		FROM delete_requests WHERE book_id = ? AND user_id = ? AND status = 'pending'`, bookID, userID); err != nil {
		return nil
	}
	return &r
}

// PendingDeleteCount is how many books wait for review.
func (s *Store) PendingDeleteCount() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(DISTINCT book_id) FROM delete_requests WHERE status = 'pending' AND book_id IS NOT NULL`)
	return n
}

// PendingDeletes groups pending requests by book, oldest request first.
func (s *Store) PendingDeletes() ([]DeleteGroup, error) {
	var reqs []DeleteRequest
	if err := s.DB.Select(&reqs, `SELECT id, book_id, title, author, username, reason, status, decided_by, created_at, decided_at
		FROM delete_requests WHERE status = 'pending' AND book_id IS NOT NULL ORDER BY created_at, id`); err != nil {
		return nil, err
	}
	var out []DeleteGroup
	index := map[int64]int{}
	for _, r := range reqs {
		i, ok := index[*r.BookID]
		if !ok {
			b, err := s.BookByID(*r.BookID, nil)
			if err != nil {
				continue
			}
			g := DeleteGroup{BookID: b.ID, Title: b.Title, Author: b.Author, Formats: b.Formats, CalibreIDs: []string{}}
			copies, _ := s.BookCopies(b.ID)
			seen := map[string]bool{}
			var cats []string
			for _, c := range copies {
				if c.Source == "calibre" && c.ExternalID != "" && !seen[c.ExternalID] {
					seen[c.ExternalID] = true
					g.CalibreIDs = append(g.CalibreIDs, c.ExternalID)
				}
				if !seen["cat:"+c.CatalogName] {
					seen["cat:"+c.CatalogName] = true
					cats = append(cats, c.CatalogName)
				}
			}
			g.Catalogs = strings.Join(cats, ", ")
			index[b.ID] = len(out)
			out = append(out, g)
			i = len(out) - 1
		}
		out[i].Requests = append(out[i].Requests, r)
	}
	return out, nil
}

// PendingRequestIDs returns the pending request ids for the given books.
// Take them before deleting books: a deleted book's requests lose book_id.
func (s *Store) PendingRequestIDs(bookIDs []int64) ([]int64, error) {
	if len(bookIDs) == 0 {
		return nil, nil
	}
	q, args, err := sqlx.In(`SELECT id FROM delete_requests WHERE status = 'pending' AND book_id IN (?)`, bookIDs)
	if err != nil {
		return nil, err
	}
	var ids []int64
	err = s.DB.Select(&ids, q, args...)
	return ids, err
}

// DecideRequests closes the given pending requests.
func (s *Store) DecideRequests(requestIDs []int64, status, by string) error {
	if !DeleteStatuses[status] {
		return errors.New("unknown decision")
	}
	if len(requestIDs) == 0 {
		return nil
	}
	q, args, err := sqlx.In(`UPDATE delete_requests SET status = ?, decided_by = ?, decided_at = CURRENT_TIMESTAMP
		WHERE status = 'pending' AND id IN (?)`, status, by, requestIDs)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(q, args...)
	return err
}

// DecideDeletes closes the pending requests for the given books.
func (s *Store) DecideDeletes(bookIDs []int64, status, by string) error {
	ids, err := s.PendingRequestIDs(bookIDs)
	if err != nil {
		return err
	}
	return s.DecideRequests(ids, status, by)
}

// RecentDeleteDecisions lists handled requests, newest first.
func (s *Store) RecentDeleteDecisions(limit int) ([]DeleteRequest, error) {
	var out []DeleteRequest
	err := s.DB.Select(&out, `SELECT id, book_id, title, author, username, reason, status, decided_by, created_at, decided_at
		FROM delete_requests WHERE status != 'pending' ORDER BY decided_at DESC, id DESC LIMIT ?`, limit)
	return out, err
}
