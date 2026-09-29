package store

import (
	"strings"
	"time"
)

// maxSearches is how many recent searches each person keeps.
const maxSearches = 10

// Searches returns someone's recent Library searches, newest first.
func (s *Store) Searches(userID int64) ([]string, error) {
	out := []string{}
	err := s.DB.Select(&out, `SELECT query FROM search_history WHERE user_id = ? ORDER BY searched_at DESC, rowid DESC LIMIT ?`, userID, maxSearches)
	return out, err
}

// AddSearch remembers a search (moving a repeat to the top) and forgets the
// oldest beyond the last 10.
func (s *Store) AddSearch(userID int64, query string) error {
	q := strings.Join(strings.Fields(query), " ")
	if len([]rune(q)) < 2 || len(q) > 200 {
		return nil
	}
	if _, err := s.DB.Exec(`INSERT INTO search_history (user_id, query, searched_at) VALUES (?, ?, ?)
		ON CONFLICT (user_id, query) DO UPDATE SET query = excluded.query, searched_at = excluded.searched_at`,
		userID, q, time.Now().UTC().Format("2006-01-02 15:04:05.000")); err != nil {
		return err
	}
	_, err := s.DB.Exec(`DELETE FROM search_history WHERE user_id = ? AND query NOT IN
		(SELECT query FROM search_history WHERE user_id = ? ORDER BY searched_at DESC, rowid DESC LIMIT ?)`, userID, userID, maxSearches)
	return err
}

// ClearSearches forgets someone's searches.
func (s *Store) ClearSearches(userID int64) error {
	_, err := s.DB.Exec(`DELETE FROM search_history WHERE user_id = ?`, userID)
	return err
}
