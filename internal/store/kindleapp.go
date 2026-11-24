package store

import (
	"fmt"
	"strings"
)

// The Kindle app's extras (kindle-app/): the person's Amazon store books as a private library (so they are rated and
// matched like any other), the app's list of what is on the Kindle, and the words looked up in Amazon's reader.

// StoreCatalog is the person's private library of Kindle store books, made on first use.
func (s *Store) StoreCatalog(u *User) (int64, error) {
	name := u.Username + "'s Kindle store"
	if c, err := s.CatalogByName(name); err == nil && c != nil {
		return c.ID, nil
	}
	return s.CreateOwnedCatalog(name, "drive", u.ID, true)
}

// AddStoreBook puts a store book (Amazon ID asin) in the person's store library and returns the book: an existing
// library book with that title and author, else a new one (rated like any new book).
func (s *Store) AddStoreBook(catalogID int64, title, author, asin string) (id int64, created bool, err error) {
	if id = s.MatchBook(title, author); id == 0 {
		if id, err = s.UpsertBook(title, author, "", ""); err != nil {
			return 0, false, err
		}
		created = true
	}
	return id, created, s.AddCopy(catalogID, id, "kindle:"+asin, "kindle", asin)
}

// KindleBook is one book on the Kindle as the app last reported it, with its rating when it is a library book.
type KindleBook struct {
	BookID  int64   `db:"book_id" json:"id"`
	Title   string  `db:"title" json:"title"`
	Author  string  `db:"author" json:"author"`
	Format  string  `db:"format" json:"format"` // "cloud": in the Amazon library, not downloaded
	Percent float64 `db:"percent" json:"percent"`
	Stars   float64 `db:"stars" json:"stars"` // Hardcover readers' average (0 = none)
	Level   *int    `db:"spice_level" json:"level"`
	Status  string  `db:"status" json:"status"` // the book's rating status ("analyzed", "pending", …)
	Old     bool    `db:"-" json:"old_rules"`   // rated under older rules
	Rules   int     `db:"rules_version" json:"-"`
	Model   string  `db:"analysis_model" json:"-"`
}

// KindleLibrary lists what a Kindle last reported, newest opened first, then by title.
func (s *Store) KindleLibrary(userID int64, device string) []KindleBook {
	out := []KindleBook{}
	_ = s.DB.Select(&out, `SELECT COALESCE(d.book_id, 0) AS book_id, COALESCE(NULLIF(b.title, ''), d.title) AS title,
		COALESCE(NULLIF(b.author, ''), d.author) AS author, d.format, d.percent, b.spice_level, COALESCE(b.status, '') AS status,
		COALESCE(b.rules_version, 0) AS rules_version, COALESCE(b.analysis_model, '') AS analysis_model, COALESCE(b.hc_rating, 0) AS stars
		FROM device_books d LEFT JOIN books b ON b.id = d.book_id WHERE d.user_id = ? AND d.device = ? AND d.ignored = 0
		ORDER BY d.opened_at DESC, d.title COLLATE NOCASE LIMIT 3000`, userID, device)
	for i := range out {
		out[i].Old = OldRating(&Book{Status: out[i].Status, RulesVersion: out[i].Rules, AnalysisModel: out[i].Model})
	}
	return out
}

// KindleWord is a word looked up in Amazon's reader (Vocabulary Builder), with the sentence it was in.
type KindleWord struct {
	Word  string `db:"word" json:"word"`
	Usage string `db:"usage" json:"usage"`
	At    string `db:"looked_at" json:"at"`
}

// SaveKindleWords keeps a book's looked-up words; ones sent before are not added twice. It returns how many are new.
func (s *Store) SaveKindleWords(userID, bookID int64, words []KindleWord) (int, error) {
	added := 0
	for _, w := range words {
		w.Word, w.Usage = clip(strings.TrimSpace(w.Word), 80), clip(strings.TrimSpace(w.Usage), maxHighlightText)
		if w.Word == "" {
			continue
		}
		res, err := s.DB.Exec(`INSERT OR IGNORE INTO kindle_words (user_id, book_id, word, usage, looked_at) VALUES (?, ?, ?, ?, ?)`,
			userID, bookID, w.Word, w.Usage, w.At)
		if err != nil {
			return added, fmt.Errorf("saving a looked-up word: %w", err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			added++
		}
	}
	return added, nil
}

// KindleWords are a person's looked-up words in a book, oldest first.
func (s *Store) KindleWords(userID, bookID int64) []KindleWord {
	out := []KindleWord{}
	_ = s.DB.Select(&out, `SELECT word, usage, looked_at FROM kindle_words WHERE user_id = ? AND book_id = ? ORDER BY looked_at, word`, userID, bookID)
	return out
}

// IsAppDevice says whether a device's reports come from the Kindle app (Amazon's reader), not the KOReader plugin.
func (s *Store) IsAppDevice(userID int64, device string) bool {
	var v string
	_ = s.DB.Get(&v, `SELECT plugin_version FROM device_info WHERE user_id = ? AND device = ?`, userID, device)
	return strings.HasPrefix(v, "app ")
}
