package store

// Friend accounts: an adult from outside the family who uses NovelCheck
// for Discover, free-book events, Libby, her own library and her reading
// stats, and never sees the family's copies of a book.

import "fmt"

// friendClause limits what a friend sees. Everywhere: the books in a
// library of her own, and books on Discover, in an event or on her reading
// list that no family library holds (a box set's book counts where its box
// set is). listed (Discover, events, a book's page) adds every book on a
// Discover list or in an event, owned by the family or not: the New York
// Times list is the same for everyone. Their family copies stay hidden
// (HideFamily, and the book page shows her own copies only).
func friendClause(u *User, listed bool) (string, []any) {
	const copies = `FROM catalog_books fc JOIN catalogs fk ON fk.id = fc.catalog_id
		WHERE (fc.book_id = b.id OR fc.book_id IN (SELECT fm.box_id FROM box_members fm WHERE fm.book_id = b.id))`
	clause := ` AND (EXISTS (SELECT 1 ` + copies + ` AND fk.owner_id = ?)
		OR (NOT EXISTS (SELECT 1 ` + copies + ` AND fk.name NOT IN (?, ?) AND COALESCE(fk.owner_id, 0) != ?)
			AND (EXISTS (SELECT 1 ` + copies + `) OR EXISTS (SELECT 1 FROM imported_reads fr WHERE fr.book_id = b.id AND fr.user_id = ?)))`
	if listed {
		clause += ` OR EXISTS (SELECT 1 FROM discover_items fd WHERE fd.book_id = b.id) OR EXISTS (SELECT 1 FROM event_books fe WHERE fe.book_id = b.id)`
	}
	return clause + `)`, []any{u.ID, DiscoverCatalog, EventsCatalog, u.ID, u.ID}
}

// HideFamily blanks what a book row says about the family's copies (their
// file formats and library names) when a friend is looking.
func HideFamily(u *User, b *Book) {
	if u.IsFriend() {
		b.Formats, b.Catalogs = "", ""
	}
}

// FriendOwns says which of these books are in one of the friend's own
// libraries: for her, that is what "Yours" means.
func (s *Store) FriendOwns(u *User, ids []int64) map[int64]bool {
	out := map[int64]bool{}
	if len(ids) == 0 {
		return out
	}
	args := []any{u.ID}
	for _, id := range ids {
		args = append(args, id)
	}
	var got []int64
	_ = s.DB.Select(&got, `SELECT DISTINCT cb.book_id FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id
		WHERE c.owner_id = ? AND cb.book_id IN (`+marks(len(ids))+`)`, args...)
	for _, id := range got {
		out[id] = true
	}
	return out
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
