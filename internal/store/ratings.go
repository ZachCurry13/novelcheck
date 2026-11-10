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

// RatingToPush is a star rating whose newest value has not reached the person's Hardcover.
type RatingToPush struct {
	BookID int64
	ISBN   string
	Stars  float64
}

// RatingsToPush lists up to limit ratings of books with an ISBN that Hardcover has not been sent (or was sent
// at another value).
func (s *Store) RatingsToPush(userID int64, limit int) []RatingToPush {
	var rows []struct {
		BookID int64   `db:"book_id"`
		ISBN   string  `db:"isbn"`
		Stars  float64 `db:"stars"`
	}
	_ = s.DB.Select(&rows, `SELECT r.book_id, b.isbn, r.stars FROM user_ratings r JOIN books b ON b.id = r.book_id
		LEFT JOIN hardcover_rated h ON h.user_id = r.user_id AND h.book_id = r.book_id
		WHERE r.user_id = ? AND b.isbn != '' AND (h.stars IS NULL OR h.stars != r.stars) ORDER BY r.updated_at DESC LIMIT ?`, userID, limit)
	out := make([]RatingToPush, 0, len(rows))
	for _, r := range rows {
		out = append(out, RatingToPush{r.BookID, r.ISBN, r.Stars})
	}
	return out
}

// MarkRatingPushed remembers the value sent for a book.
func (s *Store) MarkRatingPushed(userID, bookID int64, stars float64) {
	_, _ = s.DB.Exec(`INSERT INTO hardcover_rated (user_id, book_id, stars) VALUES (?, ?, ?)
		ON CONFLICT (user_id, book_id) DO UPDATE SET stars = excluded.stars`, userID, bookID, stars)
}

// KeepHardcoverRating records a rating that came from the person's Hardcover, unless they already rated the book here.
func (s *Store) KeepHardcoverRating(userID, bookID int64, stars float64) {
	if stars < 0.25 || stars > 5 {
		return
	}
	stars = math.Round(stars*4) / 4
	_, _ = s.DB.Exec(`INSERT INTO user_ratings (user_id, book_id, stars) VALUES (?, ?, ?) ON CONFLICT (user_id, book_id) DO NOTHING`, userID, bookID, stars)
	_, _ = s.DB.Exec(`INSERT INTO hardcover_rated (user_id, book_id, stars) VALUES (?, ?, ?) ON CONFLICT (user_id, book_id) DO NOTHING`, userID, bookID, stars)
}
