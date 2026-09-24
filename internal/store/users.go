package store

import (
	"database/sql"
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("not found")

const userCols = `id, username, password_hash, role, hide_open_door, hide_nudity, hide_solo_acts,
	hide_innuendo, hide_dark_occult, hide_lgbtq, hide_unrated, delivery_method, kindle_email, created_at`

func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.DB.Get(&n, `SELECT COUNT(*) FROM users`)
	return n, err
}

func (s *Store) ListUsers() ([]User, error) {
	var us []User
	err := s.DB.Select(&us, `SELECT `+userCols+` FROM users ORDER BY username`)
	return us, err
}

// ListUsersByRole returns only users with the given role.
func (s *Store) ListUsersByRole(role string) ([]User, error) {
	var us []User
	err := s.DB.Select(&us, `SELECT `+userCols+` FROM users WHERE role = ? ORDER BY username`, role)
	return us, err
}

func (s *Store) UserByID(id int64) (*User, error) {
	return s.getUser(`SELECT `+userCols+` FROM users WHERE id = ?`, id)
}

func (s *Store) UserByName(name string) (*User, error) {
	return s.getUser(`SELECT `+userCols+` FROM users WHERE username = ?`, name)
}

func (s *Store) getUser(q string, arg any) (*User, error) {
	var u User
	if err := s.DB.Get(&u, q, arg); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// CreateUser inserts a user. Restricted accounts default to the strictest
// content profile; admins can loosen it afterwards.
func (s *Store) CreateUser(username, hash, role string) (*User, error) {
	if !ValidRole(role) {
		return nil, fmt.Errorf("invalid role %q", role)
	}
	strict := role == RoleRestricted
	res, err := s.DB.Exec(`INSERT INTO users (username, password_hash, role, hide_open_door,
		hide_nudity, hide_solo_acts, hide_innuendo, hide_dark_occult, hide_unrated)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		username, hash, role, strict, strict, strict, strict, strict, strict)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.UserByID(id)
}

// UpdateUserProfile saves role, content rules and delivery preferences.
func (s *Store) UpdateUserProfile(u *User) error {
	_, err := s.DB.Exec(`UPDATE users SET role = ?, hide_open_door = ?, hide_nudity = ?,
		hide_solo_acts = ?, hide_innuendo = ?, hide_dark_occult = ?, hide_lgbtq = ?,
		hide_unrated = ?, delivery_method = ?, kindle_email = ? WHERE id = ?`,
		u.Role, u.HideOpenDoor, u.HideNudity, u.HideSoloActs, u.HideInnuendo,
		u.HideDarkOccult, u.HideLGBTQ, u.HideUnrated, u.DeliveryMethod, u.KindleEmail, u.ID)
	return err
}

// UpdateDelivery changes only a user's own delivery preferences.
func (s *Store) UpdateDelivery(id int64, method, email string) error {
	_, err := s.DB.Exec(`UPDATE users SET delivery_method = ?, kindle_email = ? WHERE id = ?`,
		method, email, id)
	return err
}

func (s *Store) SetPassword(id int64, hash string) error {
	_, err := s.DB.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
	return err
}

func (s *Store) DeleteUser(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (s *Store) CountAdmins() (int, error) {
	var n int
	err := s.DB.Get(&n, `SELECT COUNT(*) FROM users WHERE role = 'admin'`)
	return n, err
}

// --- sessions ---

func (s *Store) CreateSession(tokenHash string, userID int64, days int) error {
	_, err := s.DB.Exec(`INSERT INTO sessions (token_hash, user_id, expires_at)
		VALUES (?, ?, datetime('now', ?))`, tokenHash, userID, fmt.Sprintf("+%d days", days))
	return err
}

// SessionUser resolves a live session to its user.
func (s *Store) SessionUser(tokenHash string) (*User, error) {
	var id int64
	err := s.DB.Get(&id, `SELECT user_id FROM sessions
		WHERE token_hash = ? AND expires_at > datetime('now')`, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.UserByID(id)
}

func (s *Store) DeleteSession(tokenHash string) error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) DeleteUserSessions(userID int64) error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

func (s *Store) PurgeExpiredSessions() error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE expires_at <= datetime('now')`)
	return err
}
