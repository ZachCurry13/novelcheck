package store

import "strings"

// ServerError is one failed request: its short code (shown to the person as
// E-xxxx), when, what was asked and the real cause.
type ServerError struct {
	Code     string `db:"code" json:"code"`
	At       string `db:"at" json:"at"`
	Method   string `db:"method" json:"method"`
	Path     string `db:"path" json:"path"`
	Status   int    `db:"status" json:"status"`
	Username string `db:"username" json:"username"`
	Detail   string `db:"detail" json:"detail"`
}

const keepErrors = 300

// LogError keeps a failed request (the newest 300 are kept).
func (s *Store) LogError(e ServerError) {
	clip := func(v string, n int) string {
		if len(v) > n {
			return v[:n]
		}
		return strings.TrimSpace(v)
	}
	if _, err := s.DB.Exec(`INSERT INTO error_log (code, method, path, status, username, detail) VALUES (?, ?, ?, ?, ?, ?)`,
		e.Code, e.Method, clip(e.Path, 200), e.Status, clip(e.Username, 60), clip(e.Detail, 400)); err != nil {
		return
	}
	_, _ = s.DB.Exec(`DELETE FROM error_log WHERE id <= (SELECT COALESCE(MAX(id), 0) - ? FROM error_log)`, keepErrors)
}

// RecentErrors lists the newest failed requests.
func (s *Store) RecentErrors(n int) []ServerError {
	out := []ServerError{}
	_ = s.DB.Select(&out, `SELECT code, at, method, path, status, username, detail FROM error_log ORDER BY id DESC LIMIT ?`, n)
	return out
}

// ClearErrors empties the list.
func (s *Store) ClearErrors() error {
	_, err := s.DB.Exec(`DELETE FROM error_log`)
	return err
}
