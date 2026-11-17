package store

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
	"time"
)

// Highlights and notes from an e-reader (the KOReader plugin sends them; api/highlights.go): kept per person and
// book, shown on the book's page.

// Highlight is one marked passage, with the note written on it.
type Highlight struct {
	Key     string `db:"key" json:"key"`
	Text    string `db:"text" json:"text"`
	Note    string `db:"note" json:"note"`
	Chapter string `db:"chapter" json:"chapter"`
	Page    string `db:"page" json:"page"`
	TakenAt string `db:"taken_at" json:"taken_at"`
	Device  string `db:"device" json:"device"`
}

const maxHighlightText = 4000

// HighlightKey is what makes one highlight the same one when it is sent again.
func HighlightKey(device, takenAt, text string) string {
	sum := sha1.Sum([]byte(device + "\x00" + takenAt + "\x00" + strings.TrimSpace(text)))
	return hex.EncodeToString(sum[:10])
}

// SaveHighlights adds a person's highlights of a book, leaving the ones already there as they are except for a
// note changed on the device. It returns how many were new.
func (s *Store) SaveHighlights(userID, bookID int64, list []Highlight) (int, error) {
	before := s.HighlightCount(userID, bookID)
	for _, h := range list {
		h.Text, h.Note = clip(h.Text, maxHighlightText), clip(h.Note, maxHighlightText)
		if h.Text == "" && h.Note == "" {
			continue
		}
		h.Chapter, h.Page, h.TakenAt, h.Device = clip(h.Chapter, 200), clip(h.Page, 40), clip(h.TakenAt, 40), clip(h.Device, 60)
		if _, err := s.DB.Exec(`INSERT INTO book_highlights (user_id, book_id, key, text, note, chapter, page, taken_at, device, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(user_id, book_id, key) DO UPDATE SET note = excluded.note, chapter = excluded.chapter, page = excluded.page`,
			userID, bookID, HighlightKey(h.Device, h.TakenAt, h.Text), h.Text, h.Note, h.Chapter, h.Page, h.TakenAt, h.Device, time.Now().Unix()); err != nil {
			return 0, err
		}
	}
	return s.HighlightCount(userID, bookID) - before, nil
}

// Highlights lists a person's highlights of a book, in the order they were made.
func (s *Store) Highlights(userID, bookID int64) []Highlight {
	out := []Highlight{}
	_ = s.DB.Select(&out, `SELECT key, text, note, chapter, page, taken_at, device FROM book_highlights
		WHERE user_id = ? AND book_id = ? ORDER BY taken_at, created_at, key`, userID, bookID)
	return out
}

// HighlightCount is how many highlights a person has in a book.
func (s *Store) HighlightCount(userID, bookID int64) int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM book_highlights WHERE user_id = ? AND book_id = ?`, userID, bookID)
	return n
}

// LibraryBookForDevice is the library book for a file on a device (0 when none) and whether the person marked it
// "not a book": the same match the device report uses (their own choice, the file's fingerprint, its title and author).
func (s *Store) LibraryBookForDevice(userID int64, b DeviceBook) (int64, bool) {
	return s.findForDevice(userID, b)
}
