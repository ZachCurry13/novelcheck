package store

// Sharing a reading profile with the family. It is private until the person
// (or, for a kid, a parent) turns it on; "current" separately shows what they
// are reading right now.

// Sharing is who may look at someone's reading stats.
type Sharing struct {
	Profile bool `json:"profile"`
	Current bool `json:"current"`
}

func (s *Store) SharingOf(userID int64) Sharing {
	var sh struct {
		Profile bool `db:"share_profile"`
		Current bool `db:"share_current"`
	}
	_ = s.DB.Get(&sh, `SELECT share_profile, share_current FROM users WHERE id = ?`, userID)
	return Sharing{Profile: sh.Profile, Current: sh.Profile && sh.Current}
}

func (s *Store) SetSharing(userID int64, sh Sharing) error {
	_, err := s.DB.Exec(`UPDATE users SET share_profile = ?, share_current = ? WHERE id = ?`, sh.Profile, sh.Profile && sh.Current, userID)
	return err
}

// ReaderRow is a person on the Readers page.
type ReaderRow struct {
	ID       int64  `db:"id" json:"id"`
	Username string `db:"username" json:"username"`
	Role     string `db:"role" json:"role"`
	AgeLevel int    `db:"age_level" json:"age_level"`
	Shared   bool   `db:"share_profile" json:"shared"` // false for a kid a parent is looking at
	Current  bool   `db:"share_current" json:"-"`
}

// SharingReaders lists the people someone may look at: everyone who shares,
// and, for a parent, their kids whether or not they share.
func (s *Store) SharingReaders(viewerID int64, withKids bool) []ReaderRow {
	out := []ReaderRow{}
	_ = s.DB.Select(&out, `SELECT id, username, role, age_level, share_profile, share_current FROM users
		WHERE id != ? AND (share_profile = 1 OR (? AND role = 'restricted')) ORDER BY username COLLATE NOCASE`, viewerID, withKids)
	return out
}

// ReadingSecondsSince is how long someone read since a time (Unix seconds).
func (s *Store) ReadingSecondsSince(userID, from int64) int64 {
	var n int64
	_ = s.DB.Get(&n, `SELECT COALESCE(SUM(seconds), 0) FROM reading_hours WHERE user_id = ? AND hour >= ?`, userID, from)
	return n
}

// FinishedSince is how many books someone finished reading since a time.
func (s *Store) FinishedSince(userID, from int64) int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM koreader_books WHERE user_id = ? AND ignored = 0 AND percent >= ? AND last_open >= ?`, userID, FinishedAt, from)
	return n
}

// NowReading is the book someone opened most recently and has not finished,
// when they opened it within the last two weeks.
func (s *Store) NowReading(userID int64) (bookID int64, title string, at int64, percent float64) {
	var r struct {
		BookID  *int64  `db:"book_id"`
		Title   string  `db:"title"`
		At      int64   `db:"last_open"`
		Percent float64 `db:"percent"`
	}
	if s.DB.Get(&r, `SELECT book_id, title, last_open, percent FROM koreader_books
		WHERE user_id = ? AND ignored = 0 AND percent < ? AND last_open >= strftime('%s', 'now') - 14 * 86400
		ORDER BY last_open DESC LIMIT 1`, userID, FinishedAt) != nil || r.BookID == nil {
		return 0, "", 0, 0
	}
	return *r.BookID, r.Title, r.At, r.Percent
}
