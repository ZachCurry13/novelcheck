package store

import (
	"errors"
	"strings"
)

// Wish is a book someone would like the family to get.
type Wish struct {
	ID         int64  `db:"id" json:"id"`
	BookID     int64  `db:"book_id" json:"book_id"`
	Title      string `db:"title" json:"title"`
	Author     string `db:"author" json:"author"`
	SpiceLevel *int   `db:"spice_level" json:"spice_level"`
	Username   string `db:"username" json:"username"`
	Note       string `db:"note" json:"note"`
	Status     string `db:"status" json:"status"` // wanted | approved | acquired | declined
	DecidedBy  string `db:"decided_by" json:"decided_by"`
	CreatedAt  string `db:"created_at" json:"created_at"`
	UpdatedAt  string `db:"updated_at" json:"updated_at"`
}

// ErrAlreadyWished means the person already has this book on the wishlist.
var ErrAlreadyWished = errors.New("this book is already on your wishlist")

// ownedCond is true when the book (alias b) is in a real library, not only looked up.
const ownedCond = `EXISTS (SELECT 1 FROM catalog_books oc JOIN catalogs ocat ON ocat.id = oc.catalog_id
	WHERE oc.book_id = b.id AND ocat.name != 'Looked up')`

const wishCols = `w.id, w.book_id, b.title, b.author, b.spice_level, w.username, w.note, w.status, w.decided_by,
	w.created_at, w.updated_at`

// AddWish puts a book on the person's wishlist.
func (s *Store) AddWish(bookID int64, u *User, note string) error {
	_, err := s.DB.Exec(`INSERT INTO wishlist (book_id, user_id, username, note) VALUES (?, ?, ?, ?)`,
		bookID, u.ID, u.Username, strings.TrimSpace(note))
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return ErrAlreadyWished
	}
	return err
}

// RemoveWish takes a book off the person's wishlist (while it's still open).
func (s *Store) RemoveWish(bookID, userID int64) error {
	_, err := s.DB.Exec(`DELETE FROM wishlist WHERE book_id = ? AND user_id = ? AND status IN ('wanted', 'approved')`, bookID, userID)
	return err
}

// MyWish is the person's open wish for a book, or nil.
func (s *Store) MyWish(bookID, userID int64) *Wish {
	var w Wish
	if err := s.DB.Get(&w, `SELECT `+wishCols+` FROM wishlist w JOIN books b ON b.id = w.book_id
		WHERE w.book_id = ? AND w.user_id = ? AND w.status IN ('wanted', 'approved')`, bookID, userID); err != nil {
		return nil
	}
	return &w
}

// Wishes lists the wishlist, newest first: everyone's (for parents) or one
// person's. Books that have since arrived in a library are marked acquired.
func (s *Store) Wishes(userID int64, everyone bool) ([]Wish, error) {
	_, _ = s.DB.Exec(`UPDATE wishlist SET status = 'acquired', decided_by = 'in the library now', updated_at = CURRENT_TIMESTAMP
		WHERE status IN ('wanted', 'approved') AND book_id IN (SELECT b.id FROM books b WHERE ` + ownedCond + `)`)
	q := `SELECT ` + wishCols + ` FROM wishlist w JOIN books b ON b.id = w.book_id`
	args := []any{}
	if !everyone {
		q += ` WHERE w.user_id = ?`
		args = append(args, userID)
	}
	out := []Wish{}
	err := s.DB.Select(&out, q+` ORDER BY w.status NOT IN ('wanted', 'approved'), w.id DESC LIMIT 200`, args...)
	return out, err
}

// DecideWish approves ("to get"), declines, or marks a wish acquired.
func (s *Store) DecideWish(id int64, action, by string) (bool, error) {
	status := map[string]string{"approve": "approved", "decline": "declined", "acquired": "acquired"}[action]
	if status == "" {
		return false, errors.New("action must be approve, decline or acquired")
	}
	res, err := s.DB.Exec(`UPDATE wishlist SET status = ?, decided_by = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status IN ('wanted', 'approved')`, status, by, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// PendingWishes counts wishes waiting for a parent's decision.
func (s *Store) PendingWishes() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM wishlist WHERE status = 'wanted'`)
	return n
}
