package store

// SaveDeepProgress keeps the parts of a Deep Scan finished so far (a JSON list the scan writes and reads back).
func (s *Store) SaveDeepProgress(deepID int64, data string) {
	_, _ = s.DB.Exec(`INSERT INTO deep_progress (deep_id, data) VALUES (?, ?)
		ON CONFLICT(deep_id) DO UPDATE SET data = excluded.data, updated_at = CURRENT_TIMESTAMP`, deepID, data)
}

// DeepProgress is what SaveDeepProgress kept ("" when nothing).
func (s *Store) DeepProgress(deepID int64) string {
	var data string
	_ = s.DB.Get(&data, `SELECT data FROM deep_progress WHERE deep_id = ?`, deepID)
	return data
}

// ClearDeepProgress forgets it (the scan ended).
func (s *Store) ClearDeepProgress(deepID int64) {
	_, _ = s.DB.Exec(`DELETE FROM deep_progress WHERE deep_id = ?`, deepID)
}
