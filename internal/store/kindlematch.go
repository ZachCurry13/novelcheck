package store

import (
	"strings"

	"github.com/zachcurry13/novelcheck/internal/titles"
)

// What a person decided about a book on their e-reader: this is that library
// book, or this is not a book at all. It is kept by the file's fingerprint
// (else its name), so the choice survives every later report and sync.

type override struct {
	BookID  int64 `db:"book_id"`
	Ignored bool  `db:"ignored"`
}

func (s *Store) override(userID int64, key string) (override, bool) {
	var o override
	if s.DB.Get(&o, `SELECT book_id, ignored FROM kindle_overrides WHERE user_id = ? AND key = ?`, userID, key) != nil {
		return override{}, false
	}
	return o, true
}

func deviceKey(md5, filename string) string {
	if md5 != "" {
		return md5
	}
	return "f:" + strings.ToLower(filename)
}

func koKey(md5, title, authors string) string {
	if md5 != "" {
		return md5
	}
	return "k:" + NormKey(title, authors)
}

// findForDevice is the library book for a file on a device (0 if none) and
// whether the person marked it "not a book". Order: their own choice, the
// file's fingerprint, the book's own title and author (read by the plugin),
// then guesses from the file name.
func (s *Store) findForDevice(userID int64, b DeviceBook) (int64, bool) {
	if o, ok := s.override(userID, deviceKey(b.MD5, b.Filename)); ok {
		return o.BookID, o.Ignored
	}
	if b.MD5 != "" {
		if id := s.DocBook(b.MD5); id > 0 {
			return id, false
		}
	}
	if b.Author != "" && b.Title != "" {
		if id := s.MatchBook(b.Title, b.Author); id > 0 {
			return id, false
		}
	}
	guesses := titles.FromFilename(b.Filename)
	if b.Title != "" {
		guesses = append(guesses, titles.Guess{Title: b.Title})
	}
	for _, g := range guesses { // readings with an author first: the library must agree on the author too
		if g.Author != "" {
			if id := s.MatchBook(g.Title, g.Author); id > 0 {
				return id, false
			}
		}
	}
	for _, g := range guesses {
		if g.Author == "" {
			if id := s.MatchBook(g.Title, ""); id > 0 {
				return id, false
			}
		}
	}
	return 0, false
}

// MatchRef names one book on an e-reader.
type MatchRef struct {
	Kind     string `json:"kind"` // "device" (a file the plugin reported) or "koreader" (from the statistics)
	Device   string `json:"device"`
	Filename string `json:"filename"`
	Title    string `json:"title"`
	Authors  string `json:"authors"`
	MD5      string `json:"md5"`
}

// ApplyMatch records the person's choice for a book (bookID, or ignored for
// "not a book"; neither forgets an earlier choice) and applies it now.
func (s *Store) ApplyMatch(userID int64, ref MatchRef, bookID int64, ignored bool) error {
	key := deviceKey(ref.MD5, ref.Filename)
	if ref.Kind == "koreader" {
		key = koKey(ref.MD5, ref.Title, ref.Authors)
	}
	tx, err := s.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if bookID == 0 && !ignored {
		if _, err := tx.Exec(`DELETE FROM kindle_overrides WHERE user_id = ? AND key = ?`, userID, key); err != nil {
			return err
		}
	} else if _, err := tx.Exec(`INSERT OR REPLACE INTO kindle_overrides (user_id, key, book_id, ignored) VALUES (?, ?, ?, ?)`, userID, key, bookID, ignored); err != nil {
		return err
	}
	var id any
	if bookID > 0 {
		id = bookID
	}
	if ref.MD5 != "" { // the same file wherever it shows up
		if _, err := tx.Exec(`UPDATE device_books SET book_id = ?, ignored = ? WHERE user_id = ? AND md5 = ?`, id, ignored, userID, ref.MD5); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE koreader_books SET book_id = ?, ignored = ? WHERE user_id = ? AND md5 = ?`, id, ignored, userID, ref.MD5); err != nil {
			return err
		}
	} else if ref.Kind == "koreader" {
		if _, err := tx.Exec(`UPDATE koreader_books SET book_id = ?, ignored = ? WHERE user_id = ? AND title = ? AND authors = ?`, id, ignored, userID, ref.Title, ref.Authors); err != nil {
			return err
		}
	} else if _, err := tx.Exec(`UPDATE device_books SET book_id = ?, ignored = ? WHERE user_id = ? AND device = ? AND filename = ?`, id, ignored, userID, ref.Device, ref.Filename); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if bookID > 0 && ref.MD5 != "" {
		s.SetDocBook(ref.MD5, bookID) // progress sync finds the book by its file too
	}
	return nil
}
