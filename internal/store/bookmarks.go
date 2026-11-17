package store

import "time"

// Bookmark is a place in a book someone marked in NovelCheck's reader.
type Bookmark struct {
	CFI       string  `db:"cfi" json:"cfi"`
	Fraction  float64 `db:"fraction" json:"fraction"`
	Label     string  `db:"label" json:"label"`
	CreatedAt int64   `db:"created_at" json:"created_at"`
}

// Bookmarks lists someone's bookmarks in a book, from the start of the book.
func (s *Store) Bookmarks(userID, bookID int64) ([]Bookmark, error) {
	out := []Bookmark{}
	err := s.DB.Select(&out, `SELECT cfi, fraction, label, created_at FROM reader_bookmarks WHERE user_id = ? AND book_id = ? ORDER BY fraction`, userID, bookID)
	return out, err
}

// AddBookmark marks a place (marking the same place again keeps one).
func (s *Store) AddBookmark(userID, bookID int64, b Bookmark) error {
	_, err := s.DB.Exec(`INSERT INTO reader_bookmarks (user_id, book_id, cfi, fraction, label, created_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (user_id, book_id, cfi) DO UPDATE SET label = excluded.label`, userID, bookID, b.CFI, b.Fraction, b.Label, time.Now().Unix())
	return err
}

// RemoveBookmark takes a bookmark away.
func (s *Store) RemoveBookmark(userID, bookID int64, cfi string) error {
	_, err := s.DB.Exec(`DELETE FROM reader_bookmarks WHERE user_id = ? AND book_id = ? AND cfi = ?`, userID, bookID, cfi)
	return err
}
