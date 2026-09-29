package store

// "Same book as…": a parent says two books are one (a TV tie-in edition,
// a renamed reissue). Everything that points at either moves to the one with
// the better rating, the other's title is remembered as an alias so it never
// comes back as a separate book, and the other goes.

import (
	"errors"
	"strings"
)

// bookMoves are the links that follow a merged book. OR IGNORE: where the
// kept book already has the same link, its own stays (the other's goes with
// the other book).
var bookMoves = []string{
	`UPDATE OR IGNORE catalog_books SET book_id = ? WHERE book_id = ?`,
	`UPDATE OR IGNORE queue_items SET book_id = ? WHERE book_id = ?`,
	`UPDATE book_notes SET book_id = ? WHERE book_id = ?`,
	`UPDATE OR IGNORE event_books SET book_id = ? WHERE book_id = ?`,
	`UPDATE OR IGNORE collection_books SET book_id = ? WHERE book_id = ?`,
	`UPDATE OR IGNORE wishlist SET book_id = ? WHERE book_id = ?`,
	`UPDATE OR IGNORE delete_requests SET book_id = ? WHERE book_id = ?`,
	`UPDATE OR IGNORE cover_reports SET book_id = ? WHERE book_id = ?`,
	`UPDATE OR IGNORE discover_items SET book_id = ? WHERE book_id = ?`,
	`UPDATE OR IGNORE deep_reads SET book_id = ? WHERE book_id = ?`,
	`UPDATE token_usage SET book_id = ? WHERE book_id = ?`,
	`UPDATE book_aliases SET book_id = ? WHERE book_id = ?`,
	`UPDATE OR IGNORE shelf_rejects SET book_id = ? WHERE book_id = ?`,
}

// ratingRank puts a parent's rating first, then a Deep Scan, then any AI
// rating, then none.
func ratingRank(b *Book) int {
	switch {
	case strings.HasPrefix(b.AnalysisModel, "manual:"):
		return 3
	case strings.HasPrefix(b.AnalysisModel, DeepModelPrefix):
		return 2
	case b.Status == "analyzed":
		return 1
	}
	return 0
}

// MergeBooks makes a and b one book and returns the one kept: the better
// rated, then the one in more libraries, then a.
func (s *Store) MergeBooks(a, b int64) (int64, error) {
	if a == b {
		return 0, errors.New("that's the same book")
	}
	ba, err := s.BookByID(a, nil)
	if err != nil {
		return 0, err
	}
	bb, err := s.BookByID(b, nil)
	if err != nil {
		return 0, err
	}
	keep, other := ba, bb
	copies := func(id int64) int {
		var n int
		_ = s.DB.Get(&n, `SELECT COUNT(*) FROM catalog_books WHERE book_id = ?`, id)
		return n
	}
	if r1, r2 := ratingRank(ba), ratingRank(bb); r2 > r1 || (r2 == r1 && copies(b) > copies(a)) {
		keep, other = bb, ba
	}
	var otherKey string
	if err := s.DB.Get(&otherKey, `SELECT norm_key FROM books WHERE id = ?`, other.ID); err != nil {
		return 0, err
	}
	tx, err := s.DB.Beginx()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	for _, q := range bookMoves {
		if _, err := tx.Exec(q, keep.ID, other.ID); err != nil {
			return 0, err
		}
	}
	// What the kept book lacks, it takes from the other.
	if _, err := tx.Exec(`UPDATE books SET
		isbn = CASE WHEN isbn = '' THEN (SELECT isbn FROM books WHERE id = ?) ELSE isbn END,
		description = CASE WHEN description = '' THEN (SELECT description FROM books WHERE id = ?) ELSE description END,
		series = CASE WHEN series = '' THEN (SELECT series FROM books WHERE id = ?) ELSE series END,
		series_index = CASE WHEN series_index = 0 THEN (SELECT series_index FROM books WHERE id = ?) ELSE series_index END
		WHERE id = ?`, other.ID, other.ID, other.ID, other.ID, keep.ID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`DELETE FROM books WHERE id = ?`, other.ID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO book_aliases (norm_key, book_id) VALUES (?, ?)`, otherKey, keep.ID); err != nil {
		return 0, err
	}
	return keep.ID, tx.Commit()
}

// aliasKey is the key of the book a title was merged into, or key itself.
func (s *Store) aliasKey(key string) string {
	var target string
	if s.DB.Get(&target, `SELECT b.norm_key FROM book_aliases a JOIN books b ON b.id = a.book_id WHERE a.norm_key = ?`, key) == nil {
		return target
	}
	return key
}
