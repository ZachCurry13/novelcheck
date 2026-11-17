package store

import "time"

// What NovelCheck remembers about books that left (db/ratingmemory.go): how much, and forgetting it.

// KeyRememberRatings turns remembering on ("true", the default) or off ("false").
const KeyRememberRatings = "remember_ratings"

func init() { Defaults[KeyRememberRatings] = "true" }

// MemoryStats is how many ratings are remembered and the date of the oldest.
func (s *Store) MemoryStats() (ratings int, oldest time.Time) {
	var r struct {
		N   int   `db:"n"`
		Old int64 `db:"old"`
	}
	_ = s.DB.Get(&r, `SELECT COUNT(*) AS n, COALESCE(MIN(saved_at), 0) AS old FROM rating_memory`)
	if r.Old > 0 {
		oldest = time.Unix(r.Old, 0)
	}
	return r.N, oldest
}

// ForgetRatings forgets the remembered ratings saved before the time (the zero time: all of them) and says how many.
func (s *Store) ForgetRatings(before time.Time) int {
	cutoff := before.Unix()
	if before.IsZero() {
		cutoff = time.Now().Add(24 * time.Hour).Unix()
	}
	res, err := s.DB.Exec(`DELETE FROM rating_memory WHERE saved_at < ?`, cutoff)
	if err != nil {
		return 0
	}
	n, _ := res.RowsAffected()
	_, _ = s.DB.Exec(`DELETE FROM content_memory WHERE norm_key NOT IN (SELECT norm_key FROM rating_memory)`)
	_, _ = s.DB.Exec(`DELETE FROM flag_memory WHERE norm_key NOT IN (SELECT norm_key FROM rating_memory)`)
	return int(n)
}

// BookIDSet is the ids of every book that exists.
func (s *Store) BookIDSet() map[int64]bool {
	var ids []int64
	_ = s.DB.Select(&ids, `SELECT id FROM books`)
	out := make(map[int64]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}
