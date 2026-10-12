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
	_ = s.DB.Select(&ko, `SELECT title, authors, md5 FROM koreader_books WHERE user_id = ? AND book_id IS NULL AND ignored = 0`, userID)
	var dev []struct {
		Device, Filename, Title, Author, MD5 string
	}
	_ = s.DB.Select(&dev, `SELECT device, filename, title, author, md5 FROM device_books WHERE user_id = ? AND book_id IS NULL AND ignored = 0`, userID)
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
		id, _ := s.findForDevice(userID, DeviceBook{MD5: b.MD5, Filename: b.Filename, Title: b.Title, Author: b.Author})
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

// Attention is a book on an e-reader that is not in the family's library (no
// match, or only a looked-up one): the person can match it or mark it not a book.
type Attention struct {
	Kind     string  `db:"kind" json:"kind"`
	Device   string  `db:"device" json:"device"`
	Filename string  `db:"filename" json:"filename"`
	Title    string  `db:"title" json:"title"`
	Authors  string  `db:"authors" json:"authors"`
	MD5      string  `db:"md5" json:"md5"`
	Format   string  `db:"format" json:"format"`
	Size     int64   `db:"size" json:"size"`
	BookID   *int64  `db:"book_id" json:"book_id"`
	Percent  float64 `db:"percent" json:"percent"`
}

// KindleAttention lists those books, KOReader's own records first (they have
// the book's real title and author), a file on a device only when its
// fingerprint isn't among them.
func (s *Store) KindleAttention(userID int64) []Attention {
	var ko []Attention
	_ = s.DB.Select(&ko, `SELECT 'koreader' AS kind, '' AS device, '' AS filename, k.title, k.authors, k.md5, '' AS format, 0 AS size, k.book_id, k.percent
		FROM koreader_books k LEFT JOIN books b ON b.id = k.book_id
		WHERE k.user_id = ? AND k.ignored = 0 AND (b.id IS NULL OR NOT `+ownedCond+`) ORDER BY k.last_open DESC`, userID)
	var dev []Attention
	_ = s.DB.Select(&dev, `SELECT 'device' AS kind, d.device, d.filename, d.title, d.author AS authors, d.md5, d.format, d.size, d.book_id, 0 AS percent
		FROM device_books d LEFT JOIN books b ON b.id = d.book_id
		WHERE d.user_id = ? AND d.ignored = 0 AND (b.id IS NULL OR NOT `+ownedCond+`) ORDER BY d.title`, userID)
	seen := map[string]bool{}
	for _, a := range ko {
		if a.MD5 != "" {
			seen[a.MD5] = true
		}
	}
	for _, a := range dev {
		if a.MD5 == "" || !seen[a.MD5] {
			ko = append(ko, a)
			if a.MD5 != "" {
				seen[a.MD5] = true
			}
		}
	}
	if ko == nil {
		ko = []Attention{}
	}
	return ko
}

// UnmatchedDev is a file on a device with no library book yet.
type UnmatchedDev struct {
	Device   string `db:"device"`
	Filename string `db:"filename"`
	Title    string `db:"title"`
	Author   string `db:"author"`
	MD5      string `db:"md5"`
}

// UnmatchedDevice lists device files to look up (limit 0 lists none).
func (s *Store) UnmatchedDevice(userID int64, limit int) []UnmatchedDev {
	var out []UnmatchedDev
	if limit <= 0 {
		return out
	}
	_ = s.DB.Select(&out, `SELECT device, filename, title, author, md5 FROM device_books
		WHERE user_id = ? AND book_id IS NULL AND ignored = 0 ORDER BY reported_at DESC, title LIMIT ?`, userID, limit)
	return out
}

// SetDeviceBook links a device file to a library (or looked-up) book.
func (s *Store) SetDeviceBook(userID int64, b UnmatchedDev, bookID int64) {
	_, _ = s.DB.Exec(`UPDATE device_books SET book_id = ? WHERE user_id = ? AND device = ? AND filename = ?`, bookID, userID, b.Device, b.Filename)
	if b.MD5 != "" {
		s.SetDocBook(b.MD5, bookID)
	}
}
