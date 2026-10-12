package store

import "time"

// RematchUnmatched tries again to find library books for the KOReader books
// and device files that had no match (matching improves, and books get added
// to the library). It does the work at most every ten minutes per person and
// says how many it matched.
func (s *Store) RematchUnmatched(userID int64) int {
	if at, ok := s.rematchedAt.Load(userID); ok && time.Since(at.(time.Time)) < 10*time.Minute {
		return 0
	}
	s.rematchedAt.Store(userID, time.Now())
	var ko []struct {
		Title, Authors, MD5 string
	}
	_ = s.DB.Select(&ko, `SELECT title, authors, md5 FROM koreader_books WHERE user_id = ? AND book_id IS NULL`, userID)
	var dev []struct {
		Device, Filename, Title, MD5 string
	}
	_ = s.DB.Select(&dev, `SELECT device, filename, title, md5 FROM device_books WHERE user_id = ? AND book_id IS NULL`, userID)
	matched := 0
	for _, b := range ko {
		id := int64(0)
		if b.MD5 != "" {
			id = s.DocBook(b.MD5)
		}
		if id == 0 {
			id = s.MatchBook(b.Title, b.Authors)
		}
		if id > 0 {
			if _, err := s.DB.Exec(`UPDATE koreader_books SET book_id = ? WHERE user_id = ? AND title = ? AND authors = ? AND md5 = ?`,
				id, userID, b.Title, b.Authors, b.MD5); err == nil {
				matched++
			}
		}
	}
	for _, b := range dev {
		id := int64(0)
		if b.MD5 != "" {
			id = s.DocBook(b.MD5)
		}
		if id == 0 && b.Title != "" {
			id = s.MatchBook(b.Title, "")
		}
		if id > 0 {
			if _, err := s.DB.Exec(`UPDATE device_books SET book_id = ? WHERE user_id = ? AND device = ? AND filename = ?`,
				id, userID, b.Device, b.Filename); err == nil {
				matched++
			}
		}
	}
	return matched
}

// UnmatchedKO is a KOReader book with no library book yet.
type UnmatchedKO struct {
	Title   string `db:"title"`
	Authors string `db:"authors"`
	MD5     string `db:"md5"`
}

// UnmatchedKOReader lists the books to look up, most recently opened first.
func (s *Store) UnmatchedKOReader(userID int64, limit int) []UnmatchedKO {
	var out []UnmatchedKO
	_ = s.DB.Select(&out, `SELECT title, authors, md5 FROM koreader_books WHERE user_id = ? AND book_id IS NULL
		ORDER BY last_open DESC LIMIT ?`, userID, limit)
	return out
}

// SetKOReaderBook links a KOReader book to a library (or looked-up) book, and
// remembers the file's fingerprint so progress sync finds it too.
func (s *Store) SetKOReaderBook(userID int64, b UnmatchedKO, bookID int64) {
	_, _ = s.DB.Exec(`UPDATE koreader_books SET book_id = ? WHERE user_id = ? AND title = ? AND authors = ? AND md5 = ?`,
		bookID, userID, b.Title, b.Authors, b.MD5)
	if b.MD5 != "" {
		s.SetDocBook(b.MD5, bookID)
	}
}
