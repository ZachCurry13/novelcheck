package store

import "fmt"

// Books that leave an e-reader (the plugin's or the Kindle app's report no longer has them) wait for the person to
// choose: keep the book in NovelCheck, remove it from their own view of the library (hidden_books, undone any time),
// or remove it and ask for it to be deleted for good (a delete request; only the main admin deletes the files).
// The book's rating and cover always stay in NovelCheck either way.

// LeftBook is a library book that left an e-reader.
type LeftBook struct {
	ID     int64  `db:"book_id" json:"id"`
	Title  string `db:"title" json:"title"`
	Author string `db:"author" json:"author"`
	Device string `db:"device" json:"device"`
}

// NoteLeft records the books that were on a device before a report and aren't now, unless the person already
// removed them or asked for them to be deleted; books back on the device stop waiting. It returns the new ones.
func (s *Store) NoteLeft(userID int64, device string, before, after []int64) []LeftBook {
	now := map[int64]bool{}
	for _, id := range after {
		now[id] = true
		_, _ = s.DB.Exec(`DELETE FROM kindle_left WHERE user_id = ? AND book_id = ?`, userID, id)
	}
	var out []LeftBook
	for _, id := range before {
		if now[id] {
			continue
		}
		res, err := s.DB.Exec(`INSERT OR IGNORE INTO kindle_left (user_id, book_id, device) SELECT ?, ?, ?
			WHERE NOT EXISTS (SELECT 1 FROM hidden_books WHERE user_id = ? AND book_id = ?)
			AND NOT EXISTS (SELECT 1 FROM delete_requests WHERE user_id = ? AND book_id = ? AND status = 'pending')`,
			userID, id, device, userID, id, userID, id)
		if err != nil {
			continue
		}
		if n, _ := res.RowsAffected(); n > 0 {
			var b LeftBook
			if s.DB.Get(&b, `SELECT id AS book_id, title, author, ? AS device FROM books WHERE id = ?`, device, id) == nil {
				out = append(out, b)
			}
		}
	}
	return out
}

// DownloadedIDs are the library books whose files are on a device (a Kindle app's cloud-only books don't count).
func (s *Store) DownloadedIDs(userID int64, device string) []int64 {
	var ids []int64
	_ = s.DB.Select(&ids, `SELECT DISTINCT book_id FROM device_books WHERE user_id = ? AND device = ? AND book_id IS NOT NULL AND format != 'cloud'`, userID, device)
	return ids
}

// LeftBooks are the books still waiting for the person's choice, newest first.
func (s *Store) LeftBooks(userID int64) []LeftBook {
	out := []LeftBook{}
	_ = s.DB.Select(&out, `SELECT l.book_id, b.title, b.author, l.device FROM kindle_left l JOIN books b ON b.id = l.book_id
		WHERE l.user_id = ? ORDER BY l.left_at DESC LIMIT 200`, userID)
	return out
}

// DecideLeft applies the person's choice for a book that left their e-reader: "keep", "hide" (remove it from my
// library) or "delete" (remove it and ask for it to be deleted for good).
func (s *Store) DecideLeft(u *User, bookID int64, choice string) error {
	switch choice {
	case "keep":
	case "hide":
		if err := s.HideBook(u.ID, bookID); err != nil {
			return err
		}
	case "delete":
		b, err := s.BookByID(bookID, nil)
		if err != nil {
			return err
		}
		if err := s.RequestDelete(b, u, "Removed from the Kindle"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("choose keep, hide or delete")
	}
	_, err := s.DB.Exec(`DELETE FROM kindle_left WHERE user_id = ? AND book_id = ?`, u.ID, bookID)
	return err
}

// HideBook takes a book out of one person's view of the library.
func (s *Store) HideBook(userID, bookID int64) error {
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO hidden_books (user_id, book_id) VALUES (?, ?)`, userID, bookID)
	return err
}

// UnhideBook puts it back.
func (s *Store) UnhideBook(userID, bookID int64) error {
	_, err := s.DB.Exec(`DELETE FROM hidden_books WHERE user_id = ? AND book_id = ?`, userID, bookID)
	return err
}

// HiddenBooks are the books a person removed from their library, newest first.
func (s *Store) HiddenBooks(userID int64) []LeftBook {
	out := []LeftBook{}
	_ = s.DB.Select(&out, `SELECT h.book_id, b.title, b.author, '' AS device FROM hidden_books h JOIN books b ON b.id = h.book_id
		WHERE h.user_id = ? ORDER BY h.hidden_at DESC LIMIT 500`, userID)
	return out
}
