package store

import (
	"regexp"
)

var digits = regexp.MustCompile(`[0-9]+`)

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
	// The cover finder's same note again (only its numbers differ: the same books still missing) updates the newest one
	// instead of adding a line, so the log shows what changed and not ten copies of "89 of 89". Other entries (a refused
	// removal, say) keep every line: their numbers are different books.
	var last struct {
		ID   int64  `db:"id"`
		Text string `db:"text"`
	}
	if action == "Cover finder" && s.DB.Get(&last, `SELECT id, text FROM cleanup_log WHERE action = ? ORDER BY id DESC LIMIT 1`, clip(action, 120)) == nil &&
		digits.ReplaceAllString(last.Text, "#") == digits.ReplaceAllString(clip(text, 4000), "#") {
		_, _ = s.DB.Exec(`UPDATE cleanup_log SET at = CURRENT_TIMESTAMP, text = ? WHERE id = ?`, clip(text, 4000), last.ID)
		return
	}
	_, _ = s.DB.Exec(`INSERT INTO cleanup_log (action, text) VALUES (?, ?)`, clip(action, 120), clip(text, 4000))
	_, _ = s.DB.Exec(`DELETE FROM cleanup_log WHERE id NOT IN (SELECT id FROM cleanup_log ORDER BY id DESC LIMIT 500)`)
}

// CleanupLog is the newest entries first.
func (s *Store) CleanupLog(limit int) []CleanupEntry {
	out := []CleanupEntry{}
	_ = s.DB.Select(&out, `SELECT at, action, text FROM cleanup_log ORDER BY id DESC LIMIT ?`, limit)
	return out
}
