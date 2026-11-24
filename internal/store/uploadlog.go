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
