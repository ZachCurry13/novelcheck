package store

// Library cleanup's log (cleanup_log): what an action didn't do and why, newest 500 kept, shown on the bug link so a
// cleanup problem can be seen and fixed without anyone describing it.

// CleanupEntry is one line of it.
type CleanupEntry struct {
	At     string `db:"at" json:"at"`
	Action string `db:"action" json:"action"`
	Text   string `db:"text" json:"text"`
}

// AddCleanupLog notes what an action didn't do.
func (s *Store) AddCleanupLog(action, text string) {
	_, _ = s.DB.Exec(`INSERT INTO cleanup_log (action, text) VALUES (?, ?)`, clip(action, 120), clip(text, 4000))
	_, _ = s.DB.Exec(`DELETE FROM cleanup_log WHERE id NOT IN (SELECT id FROM cleanup_log ORDER BY id DESC LIMIT 500)`)
}

// CleanupLog is the newest entries first.
func (s *Store) CleanupLog(limit int) []CleanupEntry {
	out := []CleanupEntry{}
	_ = s.DB.Select(&out, `SELECT at, action, text FROM cleanup_log ORDER BY id DESC LIMIT ?`, limit)
	return out
}
