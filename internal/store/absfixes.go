package store

import "time"

// The Audiobookshelf cleanup's memory (api/absclean.go): which audiobooks were matched lately, and the fixes
// that wait for a parent.

// ABSFix is a fix waiting for a parent; Fix is audiobookshelf.Fix as JSON.
type ABSFix struct {
	ItemID string `db:"item_id" json:"item_id"`
	Title  string `db:"title" json:"title"`
	Author string `db:"author" json:"author"`
	Fix    string `db:"fix" json:"-"`
	Why    string `db:"why" json:"why"`
}

// ABSCheckedSince is the audiobooks matched after t.
func (s *Store) ABSCheckedSince(t time.Time) map[string]bool {
	var ids []string
	_ = s.DB.Select(&ids, `SELECT item_id FROM abs_checked WHERE checked_at >= ?`, t.Unix())
	out := map[string]bool{}
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// MarkABSChecked notes that an audiobook was matched now.
func (s *Store) MarkABSChecked(id string) {
	_, _ = s.DB.Exec(`INSERT INTO abs_checked (item_id, checked_at) VALUES (?, ?)
		ON CONFLICT(item_id) DO UPDATE SET checked_at = excluded.checked_at`, id, time.Now().Unix())
}

// SaveABSFix keeps (or replaces) a fix for a parent to look at.
func (s *Store) SaveABSFix(f ABSFix) {
	_, _ = s.DB.Exec(`INSERT INTO abs_fixes (item_id, title, author, fix, why, created_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(item_id) DO UPDATE SET title = excluded.title, author = excluded.author, fix = excluded.fix, why = excluded.why`,
		f.ItemID, f.Title, f.Author, f.Fix, f.Why, time.Now().Unix())
}

// ABSFixes lists the waiting fixes, oldest first.
func (s *Store) ABSFixes() ([]ABSFix, error) {
	out := []ABSFix{}
	err := s.DB.Select(&out, `SELECT item_id, title, author, fix, why FROM abs_fixes ORDER BY created_at, item_id`)
	return out, err
}

// ABSFixByID is one waiting fix.
func (s *Store) ABSFixByID(id string) (ABSFix, bool) {
	var f ABSFix
	err := s.DB.Get(&f, `SELECT item_id, title, author, fix, why FROM abs_fixes WHERE item_id = ?`, id)
	return f, err == nil
}

// DropABSFix forgets a waiting fix (applied or skipped). A skipped one isn't offered again for six months.
func (s *Store) DropABSFix(id string, skipped bool) {
	_, _ = s.DB.Exec(`DELETE FROM abs_fixes WHERE item_id = ?`, id)
	if skipped {
		_, _ = s.DB.Exec(`INSERT INTO abs_checked (item_id, checked_at) VALUES (?, ?)
			ON CONFLICT(item_id) DO UPDATE SET checked_at = excluded.checked_at`, id, time.Now().Add(150*24*time.Hour).Unix())
	}
}
