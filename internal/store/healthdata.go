package store

// What the Health page reads from the database.

// DeviceSeen is when a person's e-reader last reported.
type DeviceSeen struct {
	UserID   int64  `db:"user_id" json:"user_id"`
	User     string `db:"username" json:"user"`
	Device   string `db:"device" json:"device"`
	Version  string `db:"plugin_version" json:"version"`
	Reported int64  `db:"reported_at" json:"reported_at"`
}

// AllDevices lists every e-reader that ever reported, newest report first.
func (s *Store) AllDevices() []DeviceSeen {
	out := []DeviceSeen{}
	_ = s.DB.Select(&out, `SELECT i.user_id, u.username, i.device, i.plugin_version, i.reported_at FROM device_info i JOIN users u ON u.id = i.user_id ORDER BY i.reported_at DESC`)
	return out
}

// ErrorsSince counts failed requests and background failures logged since a time ("2006-01-02 15:04:05", UTC).
func (s *Store) ErrorsSince(at string) int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM error_log WHERE code != 'SLOW' AND at >= ?`, at)
	return n
}

// CountReviewRatings is how many books are waiting for a person to look at their rating.
func (s *Store) CountReviewRatings() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM books b WHERE b.status = 'review' AND `+ownedCond)
	return n
}

// ForgetDevice drops what NovelCheck knows of a person's e-reader (its report, its book list and its link to another
// report). Its backup files are not touched.
func (s *Store) ForgetDevice(userID int64, device string) error {
	for _, q := range []string{
		`DELETE FROM device_books WHERE user_id = ? AND device = ?`,
		`DELETE FROM device_info WHERE user_id = ? AND device = ?`,
		`DELETE FROM device_links WHERE user_id = ? AND device = ?`,
		`DELETE FROM kindle_tasks WHERE user_id = ? AND device = ? AND status = 'waiting'`,
	} {
		if _, err := s.DB.Exec(q, userID, device); err != nil {
			return err
		}
	}
	return nil
}
