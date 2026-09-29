package store

import (
	"fmt"
	"time"

	"github.com/zachcurry13/novelcheck/internal/seasons"
)

// seasonTTL is how long a shelf's matches are trusted even when the books
// look unchanged (a description can change without a new book or rating).
const seasonTTL = time.Hour

type seasonStamp struct {
	print string
	at    time.Time
}

// libraryPrint changes whenever books are added, removed or rated.
func (s *Store) libraryPrint() string {
	var p struct {
		N     int64  `db:"n"`
		Max   int64  `db:"mx"`
		Rated string `db:"rated"`
	}
	_ = s.DB.Get(&p, `SELECT COUNT(*) AS n, COALESCE(MAX(id), 0) AS mx, COALESCE(MAX(analyzed_at), '') AS rated FROM books`)
	return fmt.Sprintf("%d/%d/%s", p.N, p.Max, p.Rated)
}

// seasonReady makes sure season_books holds the shelf's current matches.
func (s *Store) seasonReady(se seasons.Season) error {
	s.seasonMu.Lock()
	defer s.seasonMu.Unlock()
	if s.seasonAt == nil {
		s.seasonAt = map[string]seasonStamp{}
	}
	print := s.libraryPrint()
	if st, ok := s.seasonAt[se.Key]; ok && st.print == print && time.Since(st.at) < seasonTTL {
		return nil
	}
	cond, args := se.WordsCond()
	tx, err := s.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM season_books WHERE season = ?`, se.Key); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO season_books (season, book_id) SELECT ?, b.id FROM books b WHERE `+cond, append([]any{se.Key}, args...)...); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.seasonAt[se.Key] = seasonStamp{print: print, at: time.Now()}
	return nil
}

// WarmSeasons gets the shelves in season ready ahead of the first visit.
func (s *Store) WarmSeasons(now time.Time) {
	for _, se := range seasons.Current(now) {
		_ = s.seasonReady(se)
	}
}
