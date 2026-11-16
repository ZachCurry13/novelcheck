package store

// Book names as Open Library writes them: the name finder (api/namefinder.go)
// goes through books that did not come from Calibre once each and gives them
// their true title and author when it is sure it is the same book.

// NameToCheck is a book the name finder has not looked at yet.
type NameToCheck struct {
	ID     int64  `db:"id"`
	Title  string `db:"title"`
	Author string `db:"author"`
	ISBN   string `db:"isbn"`
}

// NamesToCheck lists up to limit books with no Calibre copy that the name
// finder has not checked (name_checked = 0; 1 = checked, 2 = a parent typed it).
func (s *Store) NamesToCheck(limit int) ([]NameToCheck, error) {
	var out []NameToCheck
	err := s.DB.Select(&out, `SELECT b.id, b.title, b.author, b.isbn FROM books b
		WHERE b.name_checked = 0 AND EXISTS (SELECT 1 FROM catalog_books cb WHERE cb.book_id = b.id)
		AND NOT EXISTS (SELECT 1 FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id WHERE cb.book_id = b.id AND c.source = 'calibre')
		ORDER BY b.id DESC LIMIT ?`, limit)
	return out, err
}

// MarkName records that a book's name was checked (1) or typed by a parent (2).
func (s *Store) MarkName(id int64, how int) {
	_, _ = s.DB.Exec(`UPDATE books SET name_checked = ? WHERE id = ?`, how, id)
}
