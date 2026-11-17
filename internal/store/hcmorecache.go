package store

import "time"

// HCMoreGet is the kept "From Hardcover" answer for a Hardcover book id, with when it was made.
func (s *Store) HCMoreGet(hcID int) (data string, at time.Time, ok bool) {
	var row struct {
		Data string `db:"data"`
		At   string `db:"at"`
	}
	if s.DB.Get(&row, `SELECT data, at FROM hc_more_cache WHERE hc_id = ?`, hcID) != nil {
		return "", time.Time{}, false
	}
	t, err := time.Parse("2006-01-02 15:04:05", row.At)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, row.At)
	}
	return row.Data, t, true
}

// HCMoreSave keeps the answer (replacing an older one).
func (s *Store) HCMoreSave(hcID int, data string) {
	_, _ = s.DB.Exec(`INSERT INTO hc_more_cache (hc_id, data, at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(hc_id) DO UPDATE SET data = excluded.data, at = CURRENT_TIMESTAMP`, hcID, data)
}
