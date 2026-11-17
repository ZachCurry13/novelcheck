package store

// BriefBook is a library book in a few fields, for background jobs that go through every book.
type BriefBook struct {
	ID     int64  `db:"id" json:"id"`
	Title  string `db:"title" json:"title"`
	Author string `db:"author" json:"author"`
	ISBN   string `db:"isbn" json:"isbn"`
	ASIN   string `db:"asin" json:"asin,omitempty"` // an Amazon ASIN, for a free book listed with one
}

// LibraryBooks lists the books that are in a library (not only on a list or in Discover).
func (s *Store) LibraryBooks() ([]BriefBook, error) {
	out := []BriefBook{}
	err := s.DB.Select(&out, `SELECT b.id, b.title, b.author, b.isbn FROM books b WHERE `+ownedCond+` AND NOT `+discoverOnlyCond+` ORDER BY b.id`)
	return out, err
}

// FreeBooks lists the books on an event that has not ended or been archived (the free books of the day: Stuff Your
// Kindle, BookBub, Free right now, a pasted list) and that are not in a library, newest first, each with the Amazon
// ASIN its list gave. The cover finder looks for their covers too.
func (s *Store) FreeBooks() ([]BriefBook, error) {
	out := []BriefBook{}
	err := s.DB.Select(&out, `SELECT b.id, b.title, b.author, b.isbn, COALESCE(MAX(eb.asin), '') AS asin
		FROM books b JOIN event_books eb ON eb.book_id = b.id JOIN events e ON e.id = eb.event_id
		WHERE e.archived_at = '' AND NOT (`+ownedCond+`)
		GROUP BY b.id ORDER BY b.id DESC`)
	return out, err
}
