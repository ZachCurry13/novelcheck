package store

import (
	"strings"
	"time"
)

// Hardcover sync (pull): what someone is reading and has read on Hardcover
// becomes progress in NovelCheck, and moves books already in their Nightstand.

// KeyHardcoverSynced is when (RFC 3339) a person's Hardcover last synced.
func hardcoverSyncedKey(userID int64) string { return "hardcover_synced_" + itoa64(userID) }

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

// HardcoverToken is someone's saved token ("" = not connected).
func (s *Store) HardcoverToken(userID int64) string {
	var t string
	_ = s.DB.Get(&t, `SELECT hardcover_token FROM users WHERE id = ?`, userID)
	return t
}

// SetHardcoverToken saves ("" removes) the token; removing also forgets what was synced.
func (s *Store) SetHardcoverToken(userID int64, token string) error {
	if _, err := s.DB.Exec(`UPDATE users SET hardcover_token = ? WHERE id = ?`, token, userID); err != nil {
		return err
	}
	if token == "" {
		_, err := s.DB.Exec(`DELETE FROM hardcover_books WHERE user_id = ?`, userID)
		return err
	}
	return nil
}

// HardcoverUsers lists the people with a token.
func (s *Store) HardcoverUsers() []int64 {
	ids := []int64{}
	_ = s.DB.Select(&ids, `SELECT id FROM users WHERE hardcover_token != ''`)
	return ids
}

// HardcoverBook is one book from someone's Hardcover shelves.
type HardcoverBook struct {
	Title, Authors string
	ISBNs          []string
	Status         int
	Percent        float64
	UpdatedAt      int64
}

// matchHardcover finds the library book for a Hardcover one: by ISBN, else
// by title and author.
func (s *Store) matchHardcover(b HardcoverBook) int64 {
	for _, isbn := range b.ISBNs {
		var id int64
		if isbn != "" && s.DB.Get(&id, `SELECT id FROM books WHERE isbn = ? LIMIT 1`, isbn) == nil {
			return id
		}
	}
	return s.MatchBook(b.Title, b.Authors)
}

// SaveHardcover replaces someone's Hardcover books with a new sync's and
// moves their Nightstand along: a book there that was started (or finished) on
// Hardcover since it was queued becomes Reading (or Finished). It never adds
// books to your Nightstand and says how many matched the library.
func (s *Store) SaveHardcover(userID int64, books []HardcoverBook) (int, error) {
	ids := make([]int64, len(books))
	for i, b := range books {
		ids[i] = s.matchHardcover(b)
	}
	tx, err := s.DB.Beginx()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM hardcover_books WHERE user_id = ?`, userID); err != nil {
		return 0, err
	}
	matched := 0
	for i, b := range books {
		var bookID any
		if ids[i] > 0 {
			bookID = ids[i]
			matched++
		}
		if _, err := tx.Exec(`INSERT OR REPLACE INTO hardcover_books (user_id, title, authors, isbns, book_id, status, percent, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, userID, b.Title, b.Authors, strings.Join(b.ISBNs, ","), bookID, b.Status, b.Percent, b.UpdatedAt); err != nil {
			return 0, err
		}
		if ids[i] == 0 {
			continue
		}
		status := "reading"
		if b.Status == 3 {
			status = "finished"
		}
		if _, err := tx.Exec(`UPDATE queue_items SET status = ?, updated_at = CURRENT_TIMESTAMP, delivery_note = 'Hardcover'
			WHERE user_id = ? AND book_id = ? AND CAST(strftime('%s', updated_at) AS INTEGER) < ?
			AND (status = 'queued' OR (status = 'reading' AND ? = 'finished'))`, status, userID, ids[i], b.UpdatedAt, status); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	_ = s.SetSetting(hardcoverSyncedKey(userID), time.Now().UTC().Format(time.RFC3339))
	return matched, nil
}

// HardcoverStatus is what Profile shows: when it last synced and how many
// books it holds, matched or not.
func (s *Store) HardcoverStatus(userID int64) (synced string, books, matched int) {
	synced = s.Setting(hardcoverSyncedKey(userID))
	_ = s.DB.Get(&books, `SELECT COUNT(*) FROM hardcover_books WHERE user_id = ?`, userID)
	_ = s.DB.Get(&matched, `SELECT COUNT(*) FROM hardcover_books WHERE user_id = ? AND book_id IS NOT NULL`, userID)
	return
}

// LibbyLibrary is the person's library at OverDrive/Libby: its key and name ("" = none).
func (s *Store) LibbyLibrary(userID int64) (key, name string) {
	var row struct {
		Key  string `db:"libby_library"`
		Name string `db:"libby_name"`
	}
	_ = s.DB.Get(&row, `SELECT libby_library, libby_name FROM users WHERE id = ?`, userID)
	return row.Key, row.Name
}

// SetLibbyLibrary saves ("" clears) the person's library.
func (s *Store) SetLibbyLibrary(userID int64, key, name string) error {
	_, err := s.DB.Exec(`UPDATE users SET libby_library = ?, libby_name = ? WHERE id = ?`, key, name, userID)
	return err
}

// HardcoverPush reports whether the person chose to send progress to Hardcover.
func (s *Store) HardcoverPush(userID int64) bool {
	var on bool
	_ = s.DB.Get(&on, `SELECT hardcover_push FROM users WHERE id = ?`, userID)
	return on
}

// SetHardcoverPush turns sending progress on or off.
func (s *Store) SetHardcoverPush(userID int64, on bool) error {
	_, err := s.DB.Exec(`UPDATE users SET hardcover_push = ? WHERE id = ?`, on, userID)
	return err
}

// HardcoverToPush is a book whose state may need sending: reading or finished
// on your Nightstand within the last 14 days, with an ISBN, and not at Hardcover in
// that state already.
type HardcoverToPush struct {
	BookID   int64
	ISBN     string
	Status   int // 2 reading, 3 read
	Percent  float64
	FromHere bool // the newest progress isn't from Hardcover itself
}

// HardcoverPending lists up to limit books to send, newest first.
func (s *Store) HardcoverPending(userID int64, limit int) []HardcoverToPush {
	var rows []struct {
		BookID int64  `db:"book_id"`
		ISBN   string `db:"isbn"`
		Status string `db:"status"`
		Sent   int    `db:"sent_status"`
		Pages  int    `db:"sent_pages"`
	}
	_ = s.DB.Select(&rows, `SELECT q.book_id, b.isbn, q.status, COALESCE(p.status, 0) AS sent_status, COALESCE(p.pages, 0) AS sent_pages
		FROM queue_items q JOIN books b ON b.id = q.book_id LEFT JOIN hardcover_pushed p ON p.user_id = q.user_id AND p.book_id = q.book_id
		WHERE q.user_id = ? AND q.status IN ('reading', 'finished') AND b.isbn != ''
		AND q.updated_at >= datetime('now', '-14 days') ORDER BY q.updated_at DESC LIMIT ?`, userID, limit*3)
	progress := s.ReadingProgress(userID)
	var out []HardcoverToPush
	for _, r := range rows {
		status, pct := 2, 0.0
		pr, ok := progress[r.BookID]
		if r.Status == "finished" {
			status, pct = 3, 1
		} else if ok {
			pct = pr.Percent
		}
		if ok && pr.Device == "Hardcover" && status == 2 {
			continue // Hardcover is already where the reader is
		}
		if r.Sent == status && (status == 3 || int(pct*100) == r.Pages) {
			continue
		}
		out = append(out, HardcoverToPush{BookID: r.BookID, ISBN: r.ISBN, Status: status, Percent: pct, FromHere: true})
		if len(out) >= limit {
			break
		}
	}
	return out
}

// MarkHardcoverPushed remembers what was sent for a book (pages holds the percent sent).
func (s *Store) MarkHardcoverPushed(userID, bookID int64, status int, percent float64) {
	_, _ = s.DB.Exec(`INSERT INTO hardcover_pushed (user_id, book_id, status, pages) VALUES (?, ?, ?, ?)
		ON CONFLICT (user_id, book_id) DO UPDATE SET status = excluded.status, pages = excluded.pages`, userID, bookID, status, int(percent*100))
}
