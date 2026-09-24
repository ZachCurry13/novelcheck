package store

import (
	"database/sql"
	"errors"
	"strings"
)

// BookNote is a parent's note on a book, e.g. after reading it.
type BookNote struct {
	ID         int64  `db:"id" json:"id"`
	BookID     int64  `db:"book_id" json:"book_id"`
	UserID     int64  `db:"user_id" json:"user_id"`
	Author     string `db:"author" json:"author"`
	Body       string `db:"body" json:"body"`
	Visibility string `db:"visibility" json:"visibility"` // everyone | parents
	CreatedAt  string `db:"created_at" json:"created_at"`
	UpdatedAt  string `db:"updated_at" json:"updated_at"`
}

const MaxNoteLen = 4000

func validNote(body, visibility string) (string, error) {
	body = strings.TrimSpace(body)
	if body == "" || len(body) > MaxNoteLen {
		return "", errors.New("a note must be 1–4000 characters")
	}
	if visibility != "everyone" && visibility != "parents" {
		return "", errors.New("visibility must be everyone or parents")
	}
	return body, nil
}

// BookNotes returns a book's notes, oldest first. Kids only see notes shared
// with everyone.
func (s *Store) BookNotes(bookID int64, viewer *User) ([]BookNote, error) {
	q := `SELECT n.id, n.book_id, n.user_id, COALESCE(u.username, '') AS author, n.body, n.visibility,
		n.created_at, n.updated_at FROM book_notes n LEFT JOIN users u ON u.id = n.user_id
		WHERE n.book_id = ?`
	if viewer == nil || !viewer.CanManage() {
		q += ` AND n.visibility = 'everyone'`
	}
	var notes []BookNote
	err := s.DB.Select(&notes, q+` ORDER BY n.created_at, n.id`, bookID)
	if notes == nil {
		notes = []BookNote{}
	}
	return notes, err
}

func (s *Store) AddNote(bookID, userID int64, body, visibility string) (int64, error) {
	body, err := validNote(body, visibility)
	if err != nil {
		return 0, err
	}
	res, err := s.DB.Exec(`INSERT INTO book_notes (book_id, user_id, body, visibility) VALUES (?, ?, ?, ?)`,
		bookID, userID, body, visibility)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// NoteByID loads one note (for permission checks).
func (s *Store) NoteByID(id int64) (*BookNote, error) {
	var n BookNote
	err := s.DB.Get(&n, `SELECT n.id, n.book_id, n.user_id, COALESCE(u.username, '') AS author, n.body,
		n.visibility, n.created_at, n.updated_at FROM book_notes n LEFT JOIN users u ON u.id = n.user_id
		WHERE n.id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &n, err
}

func (s *Store) UpdateNote(id int64, body, visibility string) error {
	body, err := validNote(body, visibility)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`UPDATE book_notes SET body = ?, visibility = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, body, visibility, id)
	return err
}

func (s *Store) DeleteNote(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM book_notes WHERE id = ?`, id)
	return err
}
