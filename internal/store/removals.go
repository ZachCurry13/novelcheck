package store

// CalibreRemoval is one book NovelCheck removed from Calibre.
type CalibreRemoval struct {
	At        string `db:"at" json:"at"` // UTC, "2026-11-12 03:04:05"
	By        string `db:"by_user" json:"by"`
	Tool      string `db:"tool" json:"tool"`
	CalibreID int    `db:"calibre_id" json:"calibre_id"`
	Title     string `db:"title" json:"title"`
}

// LogCalibreRemovals keeps a line for each book removed from Calibre, with the title Calibre had for it.
// Lines older than a year are dropped.
func (s *Store) LogCalibreRemovals(by, tool string, titles map[int]string, ids []int) error {
	tx, err := s.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.Exec(`INSERT INTO calibre_removals (by_user, tool, calibre_id, title) VALUES (?, ?, ?, ?)`, by, tool, id, titles[id]); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM calibre_removals WHERE at < datetime('now', '-1 year')`); err != nil {
		return err
	}
	return tx.Commit()
}

// CalibreRemovals lists the newest removals first (at most limit).
func (s *Store) CalibreRemovals(limit int) []CalibreRemoval {
	out := []CalibreRemoval{}
	_ = s.DB.Select(&out, `SELECT at, by_user, tool, calibre_id, title FROM calibre_removals ORDER BY id DESC LIMIT ?`, limit)
	return out
}
