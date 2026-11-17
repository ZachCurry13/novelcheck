package store

import "github.com/zachcurry13/novelcheck/internal/titles"

// KindleUnknown is a book file on someone's Kindle that NovelCheck has no book for.
type KindleUnknown struct {
	ID       int64  `db:"id" json:"id"`
	Device   string `db:"device" json:"device"`
	Path     string `db:"path" json:"path"`
	Filename string `db:"filename" json:"filename"`
	Title    string `db:"title" json:"title"`
	Author   string `db:"author" json:"author"`
	Size     int64  `db:"size" json:"size"`
}

// NoteKindleUnknown keeps a file for review (once per path), with the reading of its name that looks most like a
// title and author.
func (s *Store) NoteKindleUnknown(userID int64, device, rel, filename string, size int64) {
	title, author := "", ""
	if gs := titles.FromFilename(filename); len(gs) > 0 {
		title, author = gs[0].Title, gs[0].Author
	}
	_, _ = s.DB.Exec(`INSERT INTO kindle_unknown (user_id, device, path, filename, title, author, size) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, path) DO UPDATE SET size = excluded.size`, userID, device, rel, filename, title, author, size)
}

// KindleUnknowns lists a person's files waiting for review.
func (s *Store) KindleUnknowns(userID int64) []KindleUnknown {
	out := []KindleUnknown{}
	_ = s.DB.Select(&out, `SELECT id, device, path, filename, title, author, size FROM kindle_unknown WHERE user_id = ? ORDER BY title COLLATE NOCASE LIMIT 2000`, userID)
	return out
}

// TakeKindleUnknowns removes the listed rows of a person and returns them (the caller adds or drops them).
func (s *Store) TakeKindleUnknowns(userID int64, ids []int64) []KindleUnknown {
	var out []KindleUnknown
	for _, id := range ids {
		var k KindleUnknown
		if s.DB.Get(&k, `SELECT id, device, path, filename, title, author, size FROM kindle_unknown WHERE user_id = ? AND id = ?`, userID, id) == nil {
			out = append(out, k)
			_, _ = s.DB.Exec(`DELETE FROM kindle_unknown WHERE user_id = ? AND id = ?`, userID, id)
		}
	}
	return out
}
