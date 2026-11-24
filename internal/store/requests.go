package store

import (
	"errors"
	"strings"
	"time"
)

// Book requests: a person (usually a kid) asks for a book, a parent says yes or no. And the Home inbox's "not now".

// Request is one ask.
type Request struct {
	ID        int64  `db:"id" json:"id"`
	UserID    int64  `db:"user_id" json:"user_id"`
	User      string `db:"username" json:"user"`
	Title     string `db:"title" json:"title"`
	Author    string `db:"author" json:"author"`
	Note      string `db:"note" json:"note"`
	BookID    *int64 `db:"book_id" json:"book_id"` // the library book it is, when it is one
	Status    string `db:"status" json:"status"`   // pending | approved | declined | done
	Answer    string `db:"answer" json:"answer"`
	DecidedBy string `db:"decided_by" json:"decided_by"`
	Created   string `db:"created_at" json:"created_at"`
	Decided   string `db:"decided_at" json:"decided_at"`
}

const requestCols = `r.id, r.user_id, u.username, r.title, r.author, r.note, r.book_id, r.status, r.answer, r.decided_by, r.created_at, COALESCE(r.decided_at, '') AS decided_at`

// AddRequest records an ask; a library book with that title and author is matched to it.
func (s *Store) AddRequest(userID int64, title, author, note string) (int64, error) {
	title, author, note = strings.TrimSpace(title), strings.TrimSpace(author), strings.TrimSpace(note)
	if title == "" || len(title) > 200 || len(author) > 200 || len(note) > 500 {
		return 0, errors.New("give a title (up to 200 characters), and the author if you know it")
	}
	var dup int
	_ = s.DB.Get(&dup, `SELECT COUNT(*) FROM book_requests WHERE user_id = ? AND status = 'pending' AND LOWER(title) = LOWER(?)`, userID, title)
	if dup > 0 {
		return 0, errors.New("you already asked for that one; a parent hasn't answered yet")
	}
	var bookID any
	if id := s.MatchBook(title, author); id != 0 {
		bookID = id
	}
	res, err := s.DB.Exec(`INSERT INTO book_requests (user_id, title, author, note, book_id) VALUES (?, ?, ?, ?, ?)`, userID, title, author, note, bookID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// RequestsFor lists one person's asks, newest first.
func (s *Store) RequestsFor(userID int64) []Request {
	out := []Request{}
	_ = s.DB.Select(&out, `SELECT `+requestCols+` FROM book_requests r JOIN users u ON u.id = r.user_id WHERE r.user_id = ? ORDER BY r.id DESC LIMIT 50`, userID)
	return out
}

// OpenRequests lists the asks waiting for a parent (pending), and the approved ones that still need the book got.
func (s *Store) OpenRequests() []Request {
	out := []Request{}
	_ = s.DB.Select(&out, `SELECT `+requestCols+` FROM book_requests r JOIN users u ON u.id = r.user_id
		WHERE r.status = 'pending' OR (r.status = 'approved' AND r.book_id IS NULL) ORDER BY r.id`)
	return out
}

// Request is one ask by its id.
func (s *Store) RequestByID(id int64) (*Request, error) {
	var r Request
	if err := s.DB.Get(&r, `SELECT `+requestCols+` FROM book_requests r JOIN users u ON u.id = r.user_id WHERE r.id = ?`, id); err != nil {
		return nil, ErrNotFound
	}
	return &r, nil
}

// DecideRequest answers a pending ask (status approved or declined), or closes an approved one (done).
func (s *Store) DecideRequest(id int64, status, answer, by string) error {
	if status != "approved" && status != "declined" && status != "done" {
		return errors.New("approve, decline or mark it done")
	}
	if len(answer) > 500 {
		answer = answer[:500]
	}
	_, err := s.DB.Exec(`UPDATE book_requests SET status = ?, answer = ?, decided_by = ?, decided_at = ? WHERE id = ?`,
		status, answer, by, time.Now().UTC().Format("2006-01-02 15:04:05"), id)
	return err
}

// LinkRequest sets the library book an approved ask turned out to be.
func (s *Store) LinkRequest(id, bookID int64) {
	_, _ = s.DB.Exec(`UPDATE book_requests SET book_id = ? WHERE id = ?`, bookID, id)
}

// Snooze sets an inbox item aside for a person until a day; Snoozed lists what is set aside now.
func (s *Store) Snooze(userID int64, item string, days int) error {
	_, err := s.DB.Exec(`INSERT INTO inbox_snooze (user_id, item, until) VALUES (?, ?, ?)
		ON CONFLICT(user_id, item) DO UPDATE SET until = excluded.until`, userID, item, time.Now().UTC().AddDate(0, 0, days).Format("2006-01-02 15:04:05"))
	return err
}

func (s *Store) Snoozed(userID int64) map[string]bool {
	var items []string
	_ = s.DB.Select(&items, `SELECT item FROM inbox_snooze WHERE user_id = ? AND until > ?`, userID, time.Now().UTC().Format("2006-01-02 15:04:05"))
	out := map[string]bool{}
	for _, i := range items {
		out[i] = true
	}
	return out
}
