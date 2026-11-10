package store

import (
	"errors"
	"math"
)

// SetRating keeps a person's star rating of a book: 0.25 to 5 in quarter steps; 0 clears it.
func (s *Store) SetRating(userID, bookID int64, stars float64) error {
	if stars == 0 {
		_, err := s.DB.Exec(`DELETE FROM user_ratings WHERE user_id = ? AND book_id = ?`, userID, bookID)
		return err
	}
	if stars < 0.25 || stars > 5 {
		return errors.New("a rating is between 1 and 5 stars")
	}
	stars = math.Round(stars*4) / 4
	_, err := s.DB.Exec(`INSERT INTO user_ratings (user_id, book_id, stars) VALUES (?, ?, ?)
		ON CONFLICT(user_id, book_id) DO UPDATE SET stars = excluded.stars, updated_at = CURRENT_TIMESTAMP`, userID, bookID, stars)
	return err
}

// RatingOf is the person's rating of one book (0 when none).
func (s *Store) RatingOf(userID, bookID int64) float64 {
	var v float64
	_ = s.DB.Get(&v, `SELECT stars FROM user_ratings WHERE user_id = ? AND book_id = ?`, userID, bookID)
	return v
}

// RatingsFor returns the person's ratings of these books.
func (s *Store) RatingsFor(userID int64, ids []int64) map[int64]float64 {
	out := map[int64]float64{}
	if len(ids) == 0 {
		return out
	}
	var rows []struct {
		BookID int64   `db:"book_id"`
		Stars  float64 `db:"stars"`
	}
	_ = s.DB.Select(&rows, `SELECT book_id, stars FROM user_ratings WHERE user_id = ?`, userID)
	want := make(map[int64]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	for _, r := range rows {
		if want[r.BookID] {
			out[r.BookID] = r.Stars
		}
	}
	return out
}
