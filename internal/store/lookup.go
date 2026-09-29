package store

import (
	"strings"

	"github.com/zachcurry13/novelcheck/internal/titles"
)

// LookedUpCatalog holds books found with Check a book that aren't in any of
// the family's libraries, so checking the same book again is instant (and free).
const LookedUpCatalog = "Looked up"

// DiscoverCatalog holds books from the Discover lists (best sellers, classics)
// that aren't in any of the family's libraries. Like looked-up books they
// are rated and filtered, but they're not the family's.
const DiscoverCatalog = "Discover"

// discoverOnlyCond is true when the book (alias b) is only listed in Discover:
// it's kept out of the Library, the status counts and "Analyze batch".
const discoverOnlyCond = `(EXISTS (SELECT 1 FROM catalog_books dq JOIN catalogs dqc ON dqc.id = dq.catalog_id
	WHERE dq.book_id = b.id AND dqc.name IN ('` + DiscoverCatalog + `', '` + EventsCatalog + `')) AND NOT EXISTS (SELECT 1 FROM catalog_books dn
	JOIN catalogs dnc ON dnc.id = dn.catalog_id WHERE dn.book_id = b.id AND dnc.name NOT IN ('` + DiscoverCatalog + `', '` + EventsCatalog + `')))`

// Owned reports whether a book is in one of the family's libraries, not just
// looked up with Check a book or listed in Discover.
func (s *Store) Owned(bookID int64) bool {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM books b WHERE b.id = ? AND `+ownedCond, bookID)
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

// ReservedCatalogName reports whether name belongs to one of NovelCheck's own
// libraries (Calibre, Looked up, Discover), so nobody can create or take it.
func ReservedCatalogName(name string) bool {
	for _, r := range []string{CalibreCatalogName, LookedUpCatalog, DiscoverCatalog, EventsCatalog} {
		if strings.EqualFold(name, r) {
			return true
		}
	}
	return false
}
