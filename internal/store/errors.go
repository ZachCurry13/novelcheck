package store

import "strings"

// ErrorGroup is a set of books that failed to rate with the same error.
type ErrorGroup struct {
	Message string   `json:"message"`
	Count   int      `json:"count"`
	Titles  []string `json:"titles"` // a few examples
}

// ErrorGroups lists rating failures grouped by error message, most common first.
func (s *Store) ErrorGroups() ([]ErrorGroup, error) {
	var rows []struct {
		Message string `db:"message"`
		Count   int    `db:"n"`
		Titles  string `db:"titles"`
	}
	err := s.DB.Select(&rows, `SELECT analysis_error AS message, COUNT(*) AS n,
		COALESCE((SELECT GROUP_CONCAT(t, char(31)) FROM (SELECT title AS t FROM books x
			WHERE x.status = 'error' AND x.analysis_error = b.analysis_error ORDER BY x.title LIMIT 5)), '') AS titles
		FROM books b WHERE status = 'error' GROUP BY analysis_error ORDER BY n DESC, message LIMIT 50`)
	if err != nil {
		return nil, err
	}
	out := make([]ErrorGroup, 0, len(rows))
	for _, r := range rows {
		g := ErrorGroup{Message: r.Message, Count: r.Count, Titles: []string{}}
		if r.Titles != "" {
			g.Titles = strings.Split(r.Titles, "\x1f")
		}
		if strings.TrimSpace(g.Message) == "" {
			g.Message = "(no error message recorded)"
		}
		out = append(out, g)
	}
	return out, nil
}

// ErrorBookIDs returns failed books, all of them or only those with message.
func (s *Store) ErrorBookIDs(message string) ([]int64, error) {
	var ids []int64
	q := `SELECT id FROM books WHERE status = 'error'`
	args := []any{}
	if message != "" {
		if message == "(no error message recorded)" {
			message = ""
		}
		q += ` AND analysis_error = ?`
		args = append(args, message)
	}
	err := s.DB.Select(&ids, q+` ORDER BY id`, args...)
	return ids, err
}
