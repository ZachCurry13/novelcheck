package store

import "github.com/zachcurry13/novelcheck/internal/titles"

// LookedUpCatalog holds books found with Check a book that aren't in any of
// the family's libraries, so checking the same book again is instant (and free).
const LookedUpCatalog = "Looked up"

// Owned reports whether a book is in one of the family's libraries, not just
// looked up with Check a book.
func (s *Store) Owned(bookID int64) bool {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id
		WHERE cb.book_id = ? AND c.name != ?`, bookID, LookedUpCatalog)
	return n > 0
}

// MatchBook finds a book NovelCheck already knows: same title and author
// surname, or else the only book with that title. 0 means none.
func (s *Store) MatchBook(title, author string) int64 {
	var id int64
	if s.DB.Get(&id, `SELECT id FROM books WHERE norm_key = ?`, NormKey(title, author)) == nil {
		return id
	}
	t := squash(titles.Clean(title))
	if t == "" {
		return 0
	}
	var ids []int64 // squash leaves only letters and digits, so no LIKE wildcards
	_ = s.DB.Select(&ids, `SELECT id FROM books WHERE norm_key LIKE ? LIMIT 2`, t+"|%")
	if len(ids) == 1 {
		return ids[0]
	}
	return 0
}
