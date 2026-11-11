package store

// SaveImportedRead keeps what a list (Goodreads, StoryGraph…) says about a book the person finished: the day
// (YYYY-MM-DD, or ""), its pages and their stars. Importing the list again updates it; an empty value never
// wipes one that was there.
func (s *Store) SaveImportedRead(userID, bookID int64, finished string, pages int, rating float64) error {
	_, err := s.DB.Exec(`INSERT INTO imported_reads (user_id, book_id, finished, pages, rating) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, book_id) DO UPDATE SET
			finished = CASE WHEN excluded.finished != '' THEN excluded.finished ELSE finished END,
			pages = CASE WHEN excluded.pages > 0 THEN excluded.pages ELSE pages END,
			rating = CASE WHEN excluded.rating > 0 THEN excluded.rating ELSE rating END`,
		userID, bookID, finished, pages, rating)
	return err
}

// ImportedFinished lists the books the person's lists say they finished on a known day, newest first, in the
// same shape as Hardcover's finished books.
func (s *Store) ImportedFinished(userID int64) []HardcoverRead {
	out := []HardcoverRead{}
	_ = s.DB.Select(&out, `SELECT b.title, b.author AS authors, r.book_id, r.finished AS finished_at, r.pages, r.rating
		FROM imported_reads r JOIN books b ON b.id = r.book_id
		WHERE r.user_id = ? AND r.finished != '' ORDER BY r.finished DESC`, userID)
	return out
}
