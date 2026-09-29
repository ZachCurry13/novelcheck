package store

import "strings"

// Reading progress: how far someone is in their books, from KOReader's
// progress sync (per device, as they read) and from KOReader's reading
// statistics (every book opened on their devices, synced to NovelCheck).
// The newest word wins.

// Progress is how far someone is in a book.
type Progress struct {
	Percent float64 `json:"percent"` // 0..1
	Device  string  `json:"device"`  // where, when known ("Kindle", "KOReader")
	At      int64   `json:"at"`      // Unix seconds
}

// KOReaderBook is a book opened on someone's KOReader devices.
type KOReaderBook struct {
	Title       string  `db:"title" json:"title"`
	Authors     string  `db:"authors" json:"authors"`
	MD5         string  `db:"md5" json:"-"`
	BookID      *int64  `db:"book_id" json:"book_id"`
	Pages       int     `db:"pages" json:"pages"`
	Percent     float64 `db:"percent" json:"percent"`
	ReadSeconds int64   `db:"read_seconds" json:"read_seconds"`
	LastOpen    int64   `db:"last_open" json:"last_open"`
}

// SaveKOReaderBooks replaces someone's KOReader books with a new sync's,
// matching each to the library (by the file's fingerprint, else by title
// and author). Books already in their Up Next move along: opened since they
// were queued → Reading, and finished → Finished. It says how many were
// matched.
func (s *Store) SaveKOReaderBooks(userID int64, books []KOReaderBook) (int, error) {
	for i := range books {
		id := int64(0)
		if books[i].MD5 != "" {
			id = s.DocBook(books[i].MD5)
		}
		if id == 0 {
			id = s.MatchBook(books[i].Title, books[i].Authors)
		}
		if id > 0 {
			books[i].BookID = &id
		}
	}
	tx, err := s.DB.Beginx()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM koreader_books WHERE user_id = ?`, userID); err != nil {
		return 0, err
	}
	matched := 0
	for _, b := range books {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO koreader_books (user_id, title, authors, md5, book_id, pages, percent, read_seconds, last_open)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, userID, b.Title, b.Authors, b.MD5, b.BookID, b.Pages, b.Percent, b.ReadSeconds, b.LastOpen); err != nil {
			return 0, err
		}
		if b.BookID == nil {
			continue
		}
		matched++
		status := "reading"
		if b.Percent >= FinishedAt {
			status = "finished"
		}
		// Only books already in Up Next, opened after they were queued: a
		// statistics file holds years of reading.
		if _, err := tx.Exec(`UPDATE queue_items SET status = ?, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND book_id = ? AND CAST(strftime('%s', updated_at) AS INTEGER) < ?
			AND (status = 'queued' OR (status = 'reading' AND ? = 'finished'))`, status, userID, *b.BookID, b.LastOpen, status); err != nil {
			return 0, err
		}
	}
	return matched, tx.Commit()
}

// KOReaderBooks lists someone's KOReader books, last opened first.
func (s *Store) KOReaderBooks(userID int64, limit int) ([]KOReaderBook, error) {
	out := []KOReaderBook{}
	err := s.DB.Select(&out, `SELECT title, authors, md5, book_id, pages, percent, read_seconds, last_open
		FROM koreader_books WHERE user_id = ? ORDER BY last_open DESC, title LIMIT ?`, userID, limit)
	return out, err
}

// ReadingProgress is how far someone is in each book they've opened on a
// synced device, by book.
func (s *Store) ReadingProgress(userID int64) map[int64]Progress {
	var rows []struct {
		BookID  int64   `db:"book_id"`
		Percent float64 `db:"percent"`
		Device  string  `db:"device"`
		At      int64   `db:"at"`
	}
	_ = s.DB.Select(&rows, `SELECT d.book_id, p.percentage AS percent, p.device, p.updated_at AS at
		FROM kosync_progress p JOIN kosync_docs d ON d.document = p.document WHERE p.user_id = ?
		UNION ALL
		SELECT k.book_id, k.percent, 'KOReader' AS device, k.last_open AS at
		FROM koreader_books k WHERE k.user_id = ? AND k.book_id IS NOT NULL`, userID, userID)
	out := map[int64]Progress{}
	for _, r := range rows {
		if cur, ok := out[r.BookID]; !ok || r.At > cur.At {
			out[r.BookID] = Progress{Percent: r.Percent, Device: deviceName(r.Device), At: r.At}
		}
	}
	return out
}

// deviceName tidies KOReader's device names ("Kindle", "KindlePaperWhite5").
func deviceName(d string) string {
	d = strings.TrimSpace(d)
	if d == "" {
		return "KOReader"
	}
	return d
}
