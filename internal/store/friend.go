package store

// Friend accounts: an adult from outside the family who uses NovelCheck
// for Discover, free-book events, Libby, her own library and her reading
// stats, and never sees a book any family library holds.

import "fmt"

// friendClause limits a friend to books on Discover, in an event, in a
// library of her own, or on her own reading list (a Goodreads import), and
// leaves out every book that any other library holds (a box set's book
// counts where its box set is).
func friendClause(u *User) (string, []any) {
	const copies = `FROM catalog_books fc JOIN catalogs fk ON fk.id = fc.catalog_id
		WHERE (fc.book_id = b.id OR fc.book_id IN (SELECT fm.box_id FROM box_members fm WHERE fm.book_id = b.id))`
	clause := ` AND NOT EXISTS (SELECT 1 ` + copies + ` AND fk.name NOT IN (?, ?) AND COALESCE(fk.owner_id, 0) != ?)
		AND (EXISTS (SELECT 1 ` + copies + `) OR EXISTS (SELECT 1 FROM imported_reads fr WHERE fr.book_id = b.id AND fr.user_id = ?))`
	return clause, []any{DiscoverCatalog, EventsCatalog, u.ID, u.ID}
}

// FriendLibrary is a friend's own library (private, hers), made the first
// time it is needed: the books she claims from events go there.
func (s *Store) FriendLibrary(u *User) (int64, error) {
	var id int64
	err := s.DB.Get(&id, `SELECT id FROM catalogs WHERE owner_id = ? AND source = 'custom' ORDER BY id LIMIT 1`, u.ID)
	if err == nil {
		return id, nil
	}
	return s.CreateOwnedCatalog(fmt.Sprintf("%s's books", u.Username), "custom", u.ID, true)
}
