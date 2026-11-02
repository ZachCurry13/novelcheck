package store

// Audiobooks (from Audiobookshelf): the settings, saving one imported item as a
// book with an audio copy, and each person's own Audiobookshelf link.

import (
	"strings"

	"github.com/zachcurry13/novelcheck/internal/genres"
)

// AudioFormat is the format of an audiobook's copy (its path is "abs:<item id>").
const AudioFormat = "audio"

// AudiobookshelfCatalog is the catalog the imported audiobooks live in.
const AudiobookshelfCatalog = "Audiobookshelf"

const (
	KeyABSURL        = "abs_url"        // the Audiobookshelf server's address
	KeyABSToken      = "abs_api_token"  // an API token (secret)
	KeyABSPollHours  = "abs_poll_hours" // how often the library is read again; 0 = only on request
	KeyABSLastSync   = "abs_last_sync"
	KeyABSLastResult = "abs_last_result"
)

// AudioItem is one audiobook as Audiobookshelf describes it.
type AudioItem struct {
	ID          string
	Title       string
	Author      string
	Narrator    string
	Series      string
	SeriesIndex float64
	Description string
	ISBN        string
	Genres      []string
	DurationSec int
}

// SaveAudioItem adds (or refreshes) an audiobook: the same title as an e-book
// already in the library becomes one book with two formats.
func (s *Store) SaveAudioItem(catalogID int64, it AudioItem) (int64, error) {
	id, err := s.UpsertBook(it.Title, it.Author, it.ISBN, it.Description)
	if err != nil {
		return 0, err
	}
	keys, kind := genres.FromTags(it.Genres)
	if _, err := s.DB.Exec(`UPDATE books SET narrator = ?, duration_sec = ?,
		series = CASE WHEN series = '' THEN ? ELSE series END,
		series_index = CASE WHEN series_index = 0 THEN ? ELSE series_index END,
		genres = CASE WHEN genres = '' THEN ? ELSE genres END,
		kind = CASE WHEN kind = '' THEN ? ELSE kind END,
		genre_source = CASE WHEN genres = '' AND ? != '' THEN 'calibre' ELSE genre_source END
		WHERE id = ?`, strings.TrimSpace(it.Narrator), it.DurationSec, it.Series, it.SeriesIndex,
		genres.Join(keys), kind, genres.Join(keys), id); err != nil {
		return 0, err
	}
	return id, s.AddCopy(catalogID, id, "abs:"+it.ID, AudioFormat, it.ID)
}

// AudioItemID is the Audiobookshelf item behind a book's audio copy ("" = none).
func (s *Store) AudioItemID(bookID int64) string {
	var id string
	_ = s.DB.Get(&id, `SELECT external_id FROM catalog_books WHERE book_id = ? AND format = ? LIMIT 1`, bookID, AudioFormat)
	return id
}

// HasFormat reports whether a book has a copy in the format: "audio", or
// "ebook" (a file the family can read, not paper or a list entry).
func (s *Store) HasFormat(bookID int64, format string) bool {
	q := `SELECT COUNT(*) FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id
		WHERE cb.book_id = ? AND c.name NOT IN ('` + LookedUpCatalog + `', '` + DiscoverCatalog + `', '` + EventsCatalog + `') AND `
	if format == "audio" {
		q += `cb.format = 'audio'`
	} else {
		q += `cb.format NOT IN ('', 'list', 'paper', 'audio')`
	}
	var n int
	_ = s.DB.Get(&n, q, bookID)
	return n > 0
}

// ABSLink is a person's own Audiobookshelf sign-in (a token; never the password).
func (s *Store) ABSLink(userID int64) (token, name string) {
	var row struct {
		Token string `db:"abs_token"`
		Name  string `db:"abs_user"`
	}
	_ = s.DB.Get(&row, `SELECT abs_token, abs_user FROM users WHERE id = ?`, userID)
	return row.Token, row.Name
}

// SetABSLink saves ("" clears) the person's Audiobookshelf link.
func (s *Store) SetABSLink(userID int64, token, name string) error {
	_, err := s.DB.Exec(`UPDATE users SET abs_token = ?, abs_user = ? WHERE id = ?`, token, name, userID)
	return err
}

// AudioCount is how many audiobooks the Audiobookshelf catalog holds.
func (s *Store) AudioCount() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(DISTINCT book_id) FROM catalog_books WHERE format = ?`, AudioFormat)
	return n
}

// AudioBookIDs maps each Audiobookshelf item id to the book it belongs to.
func (s *Store) AudioBookIDs() map[string]int64 {
	var rows []struct {
		Item string `db:"external_id"`
		Book int64  `db:"book_id"`
	}
	_ = s.DB.Select(&rows, `SELECT external_id, book_id FROM catalog_books WHERE format = ?`, AudioFormat)
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.Item] = r.Book
	}
	return out
}
