package store

import (
	"database/sql"
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("not found")

const userCols = `id, username, password_hash, role, hide_open_door, hide_nudity, hide_solo_acts,
	hide_innuendo, hide_dark_occult, hide_lgbtq, hide_unrated, delivery_method, kindle_email, guide_seen, created_at`

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

// ErrSetupDone is returned when first-run setup is attempted but an account
// already exists.
var ErrSetupDone = errors.New("setup has already been completed")

// CreateFirstAdmin creates an admin only while the users table is empty. The
// check and insert are one statement, so two browsers racing through the
// setup page can't both create an admin.
func (s *Store) CreateFirstAdmin(username, hash string) (*User, error) {
	res, err := s.DB.Exec(`INSERT INTO users (username, password_hash, role)
		SELECT ?, ?, 'admin' WHERE NOT EXISTS (SELECT 1 FROM users)`, username, hash)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrSetupDone
	}
	return s.UserByName(username)
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

// SetGuideSeen records whether the user has finished the first-login guide.
func (s *Store) SetGuideSeen(id int64, seen bool) error {
	_, err := s.DB.Exec(`UPDATE users SET guide_seen = ? WHERE id = ?`, seen, id)
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

// CreateSession stores a new session that expires after lifetime seconds.
// remember=false marks a "this browser session only" sign-in.
func (s *Store) CreateSession(tokenHash string, userID int64, lifetime int, remember bool) error {
	_, err := s.DB.Exec(`INSERT INTO sessions (token_hash, user_id, expires_at, remember)
		VALUES (?, ?, datetime('now', ?), ?)`, tokenHash, userID, fmt.Sprintf("+%d seconds", lifetime), remember)
	return err
}

// Session is a live session's user plus what's needed to renew it.
type Session struct {
	User        *User
	SecondsLeft int
	Remember    bool
}

// SessionByToken resolves a live (unexpired) session.
func (s *Store) SessionByToken(tokenHash string) (*Session, error) {
	var row struct {
		UserID   int64 `db:"user_id"`
		Left     int   `db:"secs_left"`
		Remember bool  `db:"remember"`
	}
	err := s.DB.Get(&row, `SELECT user_id, remember,
		CAST(strftime('%s', expires_at) - strftime('%s', 'now') AS INTEGER) AS secs_left
		FROM sessions WHERE token_hash = ? AND expires_at > datetime('now')`, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u, err := s.UserByID(row.UserID)
	if err != nil {
		return nil, err
	}
	return &Session{User: u, SecondsLeft: row.Left, Remember: row.Remember}, nil
}

// ExtendSession pushes a session's expiry to lifetime seconds from now.
func (s *Store) ExtendSession(tokenHash string, lifetime int) error {
	_, err := s.DB.Exec(`UPDATE sessions SET expires_at = datetime('now', ?) WHERE token_hash = ?`,
		fmt.Sprintf("+%d seconds", lifetime), tokenHash)
	return err
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
