package store

// Book names as Open Library writes them: the name finder (api/namefinder.go)
// goes through the books once each. A book not from Calibre is renamed when
// it is sure; a Calibre book gets a suggested title to save in Calibre.

// NameToCheck is a book the name finder has not looked at yet.
type NameToCheck struct {
	ID      int64  `db:"id"`
	Title   string `db:"title"`
	Author  string `db:"author"`
	ISBN    string `db:"isbn"`
	Calibre bool   `db:"calibre"` // its title comes from Calibre: a true title is a suggestion to save there
}

// NamesToCheck lists up to limit books the name finder has not checked
// (name_checked = 0; 1 = checked, 2 = a parent typed it), newest first.
func (s *Store) NamesToCheck(limit int) ([]NameToCheck, error) {
	var out []NameToCheck
	err := s.DB.Select(&out, `SELECT b.id, b.title, b.author, b.isbn,
		EXISTS (SELECT 1 FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id WHERE cb.book_id = b.id AND c.source = 'calibre') AS calibre
		FROM books b WHERE b.name_checked = 0 AND EXISTS (SELECT 1 FROM catalog_books cb WHERE cb.book_id = b.id)
		ORDER BY b.id DESC LIMIT ?`, limit)
	return out, err
}

// SuggestTitle keeps the true title found for a Calibre book ("" to forget it). It is listed under Tidy
// titles in Calibre, the cover finder searches by it, and saving it in Calibre clears it.
func (s *Store) SuggestTitle(id int64, title string) {
	_, _ = s.DB.Exec(`UPDATE books SET title_suggest = ? WHERE id = ?`, title, id)
}

// TitleSuggestion is the true title found for a book, or "".
func (s *Store) TitleSuggestion(id int64) string {
	var t string
	_ = s.DB.Get(&t, `SELECT title_suggest FROM books WHERE id = ?`, id)
	return t
}

// RecheckNames has the name finder look at every book again (once, when the finder learned something new).
func (s *Store) RecheckNames() {
	_, _ = s.DB.Exec(`UPDATE books SET name_checked = 0 WHERE name_checked = 1`)
}

// MarkName records that a book's name was checked (1) or typed by a parent (2).
func (s *Store) MarkName(id int64, how int) {
	_, _ = s.DB.Exec(`UPDATE books SET name_checked = ? WHERE id = ?`, how, id)
}

// NameTyped says whether a parent typed this book's name (it is then never changed by itself).
func (s *Store) NameTyped(id int64) bool {
	var n int
	_ = s.DB.Get(&n, `SELECT name_checked FROM books WHERE id = ?`, id)
	return n == 2
}
