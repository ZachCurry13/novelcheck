package store

import (
	"strconv"
	"strings"
)

// The log of formats removed by format cleanup, for Undo. Calibre keeps the
// files in its recycle bin; NovelCheck offers Undo for UndoDays days.

// UndoDays is how long a removal can be undone from NovelCheck (calibre's
// recycle bin keeps files for 14 days unless set otherwise).
const UndoDays = 7

const undoable = `restored_at IS NULL AND removed_at >= datetime('now', '-7 days')`

// FormatRemoval is a removed format.
type FormatRemoval struct {
	ID        int64  `db:"id" json:"id"`
	BookID    *int64 `db:"book_id" json:"book_id"`
	Title     string `db:"title" json:"title"`
	CalibreID string `db:"calibre_id" json:"calibre_id"`
	Format    string `db:"format" json:"format"`
	FileName  string `db:"file_name" json:"-"`
	Size      int64  `db:"size" json:"size"`
	Batch     string `db:"batch" json:"batch"`
	RemovedAt string `db:"removed_at" json:"removed_at"`
}

// RemovalBatch is one cleanup run that can still be undone.
type RemovalBatch struct {
	Batch     string          `db:"batch" json:"batch"`
	RemovedAt string          `db:"removed_at" json:"removed_at"`
	RemovedBy string          `db:"removed_by" json:"removed_by"`
	Files     int             `db:"files" json:"files"`
	Size      int64           `db:"size" json:"size"`
	Formats   string          `db:"formats" json:"formats"`
	Items     []FormatRemoval `db:"-" json:"items"`
}

// LogFormatRemoval records a removed format.
func (s *Store) LogFormatRemoval(f CalibreFile, fileName string, size int64, batch, by string) error {
	_, err := s.DB.Exec(`INSERT INTO format_removals (book_id, title, calibre_id, format, file_name, size, batch, removed_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, f.BookID, f.Title, f.CalibreID, strings.ToUpper(f.Format), fileName, size, batch, by)
	return err
}

// RemovalBatches lists the runs that can still be undone, newest first,
// each with up to perBatch of its files.
func (s *Store) RemovalBatches(perBatch int) ([]RemovalBatch, error) {
	out := []RemovalBatch{}
	if err := s.DB.Select(&out, `SELECT batch, MIN(removed_at) AS removed_at, MAX(removed_by) AS removed_by, COUNT(*) AS files,
		COALESCE(SUM(size), 0) AS size, GROUP_CONCAT(DISTINCT format) AS formats
		FROM format_removals WHERE `+undoable+` GROUP BY batch ORDER BY MIN(removed_at) DESC`); err != nil {
		return nil, err
	}
	for i := range out {
		items, err := s.FormatRemovals(out[i].Batch, perBatch)
		if err != nil {
			return nil, err
		}
		out[i].Items = items
	}
	return out, nil
}

// FormatRemovals lists a run's files that can still be put back (limit 0 = all).
func (s *Store) FormatRemovals(batch string, limit int) ([]FormatRemoval, error) {
	q := `SELECT id, book_id, title, calibre_id, format, file_name, size, batch, removed_at
		FROM format_removals WHERE batch = ? AND ` + undoable + ` ORDER BY title COLLATE NOCASE, id`
	if limit > 0 {
		q += ` LIMIT ` + strconv.Itoa(limit)
	}
	out := []FormatRemoval{}
	err := s.DB.Select(&out, q, batch)
	return out, err
}

// FormatRemovalByID returns one removal that can still be undone.
func (s *Store) FormatRemovalByID(id int64) (*FormatRemoval, error) {
	var r FormatRemoval
	if err := s.DB.Get(&r, `SELECT id, book_id, title, calibre_id, format, file_name, size, batch, removed_at
		FROM format_removals WHERE id = ? AND `+undoable, id); err != nil {
		return nil, ErrNotFound
	}
	return &r, nil
}

// MarkRestored records that a removed format was put back.
func (s *Store) MarkRestored(id int64) error {
	_, err := s.DB.Exec(`UPDATE format_removals SET restored_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}
