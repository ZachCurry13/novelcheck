package store

// Delta scanning: a rating remembers when its book last changed in Calibre
// (the entry's last_modified or the file's modification time, whichever is
// later). If Calibre changes the book afterwards, it's offered for
// re-rating; unchanged books are never rated twice.

// currentModified is the latest change time of any copy of book b.
const currentModified = `(SELECT COALESCE(MAX(cm.modified), '') FROM catalog_books cm WHERE cm.book_id = b.id)`

// changedCond is true when book b changed in Calibre after it was rated.
const changedCond = `(b.rated_modified != '' AND b.rated_modified < ` + currentModified + `)`

// AddCalibreCopy links a Calibre file to a book, recording when it last changed.
func (s *Store) AddCalibreCopy(catalogID, bookID int64, path, format, externalID, modified string) error {
	_, err := s.DB.Exec(`INSERT INTO catalog_books (catalog_id, book_id, path, format, external_id, modified)
		VALUES (?, ?, ?, lower(?), ?, ?) ON CONFLICT (catalog_id, book_id, path) DO UPDATE SET modified = excluded.modified`,
		catalogID, bookID, path, format, externalID, modified)
	return err
}

// BaselineModified gives ratings made before delta scanning (or before a
// book had a change time) their starting point, so they aren't all "changed".
func (s *Store) BaselineModified() error {
	_, err := s.DB.Exec(`UPDATE books AS b SET rated_modified = ` + currentModified + `
		WHERE b.status = 'analyzed' AND b.rated_modified = ''`)
	return err
}

// ChangedSinceRated counts AI-rated books that changed in Calibre after rating.
func (s *Store) ChangedSinceRated() int {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM books b WHERE b.status = 'analyzed'
		AND b.analysis_model NOT LIKE 'manual:%' AND b.analysis_model NOT LIKE 'deep:%' AND `+changedCond)
	return n
}
