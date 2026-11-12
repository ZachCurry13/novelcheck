package store

// ImportedRead is a book the person finished according to a list they imported: Hardcover's shape plus how it
// was read and how many times.
type ImportedRead struct {
	HardcoverRead
	Format string `db:"format" json:"format,omitempty"` // paper, ebook or audio
	Reads  int    `db:"reads" json:"reads,omitempty"`
}

// SaveImportedRead keeps what a list (Goodreads, StoryGraph…) says about a book the person finished: the day
// (YYYY-MM-DD, or ""), its pages, their stars, the format and the times read. Importing the list again updates
// it; an empty value never wipes one that was there.
func (s *Store) SaveImportedRead(userID, bookID int64, r ImportedRead) error {
	_, err := s.DB.Exec(`INSERT INTO imported_reads (user_id, book_id, finished, pages, rating, format, reads) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, book_id) DO UPDATE SET
			finished = CASE WHEN excluded.finished != '' THEN excluded.finished ELSE finished END,
			pages = CASE WHEN excluded.pages > 0 THEN excluded.pages ELSE pages END,
			rating = CASE WHEN excluded.rating > 0 THEN excluded.rating ELSE rating END,
			format = CASE WHEN excluded.format != '' THEN excluded.format ELSE format END,
			reads = CASE WHEN excluded.reads > 0 THEN excluded.reads ELSE reads END`,
		userID, bookID, r.Finished, r.Pages, r.Rating, r.Format, r.Reads)
	return err
}

// ImportedFinished lists the books the person's lists say they finished on a known day, newest first.
func (s *Store) ImportedFinished(userID int64) []ImportedRead {
	out := []ImportedRead{}
	_ = s.DB.Select(&out, `SELECT b.title, b.author AS authors, r.book_id, r.finished AS finished_at, r.pages, r.rating, r.format, r.reads
		FROM imported_reads r JOIN books b ON b.id = r.book_id
		WHERE r.user_id = ? AND r.finished != '' ORDER BY r.finished DESC`, userID)
	return out
}

// FinishedOn is the day the person's own record says they finished a book ("" when there is none).
func (s *Store) FinishedOn(userID, bookID int64) string {
	var day string
	_ = s.DB.Get(&day, `SELECT finished FROM imported_reads WHERE user_id = ? AND book_id = ?`, userID, bookID)
	return day
}

// SetFinishedOn records that the person finished a book on a day (YYYY-MM-DD), whatever a list said; "" takes
// the book out of their finished books (the row goes, with the pages, stars and format a list brought).
func (s *Store) SetFinishedOn(userID, bookID int64, day string) error {
	if day == "" {
		_, err := s.DB.Exec(`DELETE FROM imported_reads WHERE user_id = ? AND book_id = ?`, userID, bookID)
		return err
	}
	_, err := s.DB.Exec(`INSERT INTO imported_reads (user_id, book_id, finished) VALUES (?, ?, ?)
		ON CONFLICT(user_id, book_id) DO UPDATE SET finished = excluded.finished`, userID, bookID, day)
	return err
}

// ImportedUndated lists the books a list put on the person's "read" shelf without a day, so they can give one.
func (s *Store) ImportedUndated(userID int64) []ImportedRead {
	out := []ImportedRead{}
	_ = s.DB.Select(&out, `SELECT b.title, b.author AS authors, r.book_id, r.finished AS finished_at, r.pages, r.rating, r.format, r.reads
		FROM imported_reads r JOIN books b ON b.id = r.book_id
		WHERE r.user_id = ? AND r.finished = '' ORDER BY b.title`, userID)
	return out
}
