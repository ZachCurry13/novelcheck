package store

import (
	"errors"
	"strings"
)

// ProblemReport is a problem or idea someone sent from the app.
type ProblemReport struct {
	ID        int64  `db:"id" json:"id"`
	Username  string `db:"username" json:"username"`
	Text      string `db:"text" json:"text"`
	Page      string `db:"page" json:"page"`
	Device    string `db:"device" json:"device"`
	Version   string `db:"version" json:"version"`
	CreatedAt string `db:"created_at" json:"created_at"`
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// ReportProblem saves a report from u.
func (s *Store) ReportProblem(u *User, text, page, device, version string) error {
	if text = clip(text, 4000); text == "" {
		return errors.New("describe the problem or idea first")
	}
	_, err := s.DB.Exec(`INSERT INTO problem_reports (user_id, username, text, page, device, version) VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, text, clip(page, 200), clip(device, 300), clip(version, 40))
	return err
}

// ProblemReports lists the open reports, newest first.
func (s *Store) ProblemReports() ([]ProblemReport, error) {
	out := []ProblemReport{}
	err := s.DB.Select(&out, `SELECT id, username, text, page, device, version, created_at FROM problem_reports
		WHERE status = 'open' ORDER BY created_at DESC, id DESC`)
	return out, err
}

// CountProblemReports is how many reports are waiting.
func (s *Store) CountProblemReports() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM problem_reports WHERE status = 'open'`)
	return n
}

// CloseProblemReport marks a report done; false if it wasn't open.
func (s *Store) CloseProblemReport(id int64) (bool, error) {
	res, err := s.DB.Exec(`UPDATE problem_reports SET status = 'done' WHERE id = ? AND status = 'open'`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
