package store

import "time"

// Hardcover ratings: each book's average rating from Hardcover's readers, looked up in the background
// (api/ratingloop.go) and again after a month, for the Library's "Highest rated" order and the ★ on its card.

// RatingToCheck is a book whose Hardcover rating is missing or a month old.
type RatingToCheck struct {
	ID     int64  `db:"id"`
	Title  string `db:"title"`
	Author string `db:"author"`
	ISBN   string `db:"isbn"`
}

// RatingsToCheck lists up to limit books in any library, list or event whose rating was never looked up or is
// over 30 days old, those never looked up first.
func (s *Store) RatingsToCheck(limit int) ([]RatingToCheck, error) {
	var out []RatingToCheck
	err := s.DB.Select(&out, `SELECT b.id, b.title, b.author, b.isbn FROM books b
		WHERE b.hc_rated_at < ? AND EXISTS (SELECT 1 FROM catalog_books cb WHERE cb.book_id = b.id)
		ORDER BY b.hc_rated_at, b.id DESC LIMIT ?`, time.Now().Add(-30*24*time.Hour).Unix(), limit)
	return out, err
}

// SaveHCRating keeps a book's Hardcover rating (0 and 0 when Hardcover had no rating, or no such book).
func (s *Store) SaveHCRating(id int64, rating float64, count int) {
	_, _ = s.DB.Exec(`UPDATE books SET hc_rating = ?, hc_ratings = ?, hc_rated_at = ? WHERE id = ?`, rating, count, time.Now().Unix(), id)
}

// RatingsToken is a Hardcover connection to look ratings up with: the main admin's, else any admin's, else
// anyone's in the family ("" when nobody connected Hardcover).
func (s *Store) RatingsToken() string {
	var t string
	_ = s.DB.Get(&t, `SELECT hardcover_token FROM users WHERE hardcover_token != '' ORDER BY owner DESC, role = 'admin' DESC, id LIMIT 1`)
	return t
}

// CountHCRatings is how many books have a Hardcover rating.
func (s *Store) CountHCRatings() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM books WHERE hc_rating > 0`)
	return n
}

// SetMissingISBN gives a book the ISBN found for it, when it has none; it says whether it did.
func (s *Store) SetMissingISBN(id int64, isbn string) bool {
	if isbn == "" {
		return false
	}
	res, err := s.DB.Exec(`UPDATE books SET isbn = ? WHERE id = ? AND isbn = ''`, isbn, id)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

// CountNoISBN is how many books in a library have no ISBN.
func (s *Store) CountNoISBN() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM books b WHERE b.isbn = '' AND `+ownedCond)
	return n
}
