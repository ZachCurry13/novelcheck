package store

// ReadingHour is one book's reading in one hour (UTC), from KOReader's
// statistics.
type ReadingHour struct {
	Title   string `db:"title"`
	Authors string `db:"authors"`
	MD5     string `db:"md5"`
	Hour    int64  `db:"hour"`
	Seconds int64  `db:"seconds"`
	Pages   int    `db:"pages"`
}

// SaveReadingHours replaces someone's hours with a new sync's.
func (s *Store) SaveReadingHours(userID int64, hours []ReadingHour) error {
	tx, err := s.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM reading_hours WHERE user_id = ?`, userID); err != nil {
		return err
	}
	stmt, err := tx.Preparex(`INSERT OR REPLACE INTO reading_hours (user_id, hour, title, authors, md5, seconds, pages)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, h := range hours {
		if _, err := stmt.Exec(userID, h.Hour, h.Title, h.Authors, h.MD5, h.Seconds, h.Pages); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ReadingHours is someone's reading by the hour, oldest first (at most
// limit rows, the newest).
func (s *Store) ReadingHours(userID int64, limit int) ([]ReadingHour, error) {
	out := []ReadingHour{}
	err := s.DB.Select(&out, `SELECT title, authors, md5, hour, seconds, pages FROM (
		SELECT title, authors, md5, hour, seconds, pages FROM reading_hours WHERE user_id = ? ORDER BY hour DESC LIMIT ?
	) ORDER BY hour`, userID, limit)
	return out, err
}

// HasReadingHours says whether any reading by the hour is stored for someone.
func (s *Store) HasReadingHours(userID int64) bool {
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM (SELECT 1 FROM reading_hours WHERE user_id = ? LIMIT 1)`, userID)
	return n > 0
}
