package store

// NoteUpload remembers that a person added a Calibre book by drag and drop.
func (s *Store) NoteUpload(calibreID string, userID int64) {
	_, _ = s.DB.Exec(`INSERT INTO upload_log (calibre_id, user_id) VALUES (?, ?) ON CONFLICT(calibre_id, user_id) DO UPDATE SET at = CURRENT_TIMESTAMP`, calibreID, userID)
}

// UploadedRecently says whether this person added the Calibre book within the last week.
func (s *Store) UploadedRecently(calibreID string, userID int64) bool {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM upload_log WHERE calibre_id = ? AND user_id = ? AND at > datetime('now', '-7 days')`, calibreID, userID)
	return n > 0
}

// UploadLine is one book added by drag and drop.
type UploadLine struct {
	CalibreID string `db:"calibre_id"`
	Username  string `db:"username"`
	At        string `db:"at"`
}

// RecentUploads lists the newest additions by drag and drop, with who made them.
func (s *Store) RecentUploads(n int) []UploadLine {
	out := []UploadLine{}
	_ = s.DB.Select(&out, `SELECT l.calibre_id, u.username, l.at FROM upload_log l JOIN users u ON u.id = l.user_id ORDER BY l.at DESC LIMIT ?`, n)
	return out
}

// UploadsToday is how many books this person added in the last day.
func (s *Store) UploadsToday(userID int64) int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM upload_log WHERE user_id = ? AND at > datetime('now', '-1 day')`, userID)
	return n
}
