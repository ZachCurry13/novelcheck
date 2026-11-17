package store

import "strconv"

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

// DropCalibreEntries forgets Calibre entries that were just removed on the Content server, at once, so NovelCheck's
// lists (Duplicates in Calibre, the cleanup checklists) do not offer them again while the next full read of
// Calibre is still to come. A book left with no copy anywhere goes too; a book that has other copies stays.
func (s *Store) DropCalibreEntries(calibreIDs []int) {
	if len(calibreIDs) == 0 {
		return
	}
	cat, err := s.EnsureCatalog(CalibreCatalogName, "calibre")
	if err != nil {
		return
	}
	for _, id := range calibreIDs {
		var books []int64
		_ = s.DB.Select(&books, `SELECT DISTINCT book_id FROM catalog_books WHERE catalog_id = ? AND external_id = ?`, cat, strconv.Itoa(id))
		if _, err := s.DB.Exec(`DELETE FROM catalog_books WHERE catalog_id = ? AND external_id = ?`, cat, strconv.Itoa(id)); err != nil {
			continue
		}
		for _, b := range books {
			_, _ = s.DB.Exec(`DELETE FROM books WHERE id = ? AND NOT EXISTS (SELECT 1 FROM catalog_books WHERE book_id = ?)`, b, b)
		}
	}
}
