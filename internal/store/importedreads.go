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
