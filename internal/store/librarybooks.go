package store

// BriefBook is a library book in a few fields, for background jobs that go through every book.
type BriefBook struct {
	ID     int64  `db:"id" json:"id"`
	Title  string `db:"title" json:"title"`
	Author string `db:"author" json:"author"`
	ISBN   string `db:"isbn" json:"isbn"`
}

// LibraryBooks lists the books that are in a library (not only on a list or in Discover).
func (s *Store) LibraryBooks() ([]BriefBook, error) {
	out := []BriefBook{}
	err := s.DB.Select(&out, `SELECT b.id, b.title, b.author, b.isbn FROM books b WHERE `+ownedCond+` AND NOT `+discoverOnlyCond+` ORDER BY b.id`)
	return out, err
}
