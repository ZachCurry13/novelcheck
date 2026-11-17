package store

// NewBookCounts is how many books came into the family libraries after `since` (UTC, "2006-01-02 15:04:05"): e-books
// in a Calibre library and audiobooks. Private libraries, paper shelves and wish-list entries are not counted.
func (s *Store) NewBookCounts(since string) (ebooks, audio int) {
	_ = s.DB.Get(&ebooks, `SELECT COUNT(DISTINCT cb.book_id) FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id
		WHERE c.private = 0 AND c.physical = 0 AND c.source = 'calibre' AND cb.format != ? AND cb.added_at > ?`, AudioFormat, since)
	_ = s.DB.Get(&audio, `SELECT COUNT(DISTINCT cb.book_id) FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id
		WHERE c.private = 0 AND c.physical = 0 AND cb.format = ? AND cb.added_at > ?`, AudioFormat, since)
	return
}
