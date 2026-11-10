package store

import "strings"

// AddFromLibby makes a book from a library search result (it need not be in any library of ours), keeping the
// library's cover address when the book has none. It returns the book's id.
func (s *Store) AddFromLibby(title, author, cover string) (int64, error) {
	id, err := s.UpsertBook(title, author, "", "")
	if err != nil {
		return 0, err
	}
	if cover = strings.TrimSpace(cover); strings.HasPrefix(cover, "https://") {
		_, _ = s.DB.Exec(`UPDATE books SET cover_url = ? WHERE id = ? AND cover_url = ''`, cover, id)
	}
	return id, nil
}
