package store

import "time"

// ReaderPosition is where someone is in a book in NovelCheck's reader.
type ReaderPosition struct {
	CFI       string  `db:"cfi" json:"cfi"`
	Fraction  float64 `db:"fraction" json:"fraction"`
	UpdatedAt int64   `db:"updated_at" json:"updated_at"`
}

// SaveReaderPosition keeps someone's place in a book.
func (s *Store) SaveReaderPosition(userID, bookID int64, cfi string, fraction float64) error {
	_, err := s.DB.Exec(`INSERT INTO reader_positions (user_id, book_id, cfi, fraction, updated_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (user_id, book_id) DO UPDATE SET cfi = excluded.cfi, fraction = excluded.fraction, updated_at = excluded.updated_at`,
		userID, bookID, cfi, fraction, time.Now().Unix())
	return err
}

// ReaderPositionOf is someone's place in a book (ok false when they never opened it here).
func (s *Store) ReaderPositionOf(userID, bookID int64) (ReaderPosition, bool) {
	var p ReaderPosition
	err := s.DB.Get(&p, `SELECT cfi, fraction, updated_at FROM reader_positions WHERE user_id = ? AND book_id = ?`, userID, bookID)
	return p, err == nil
}

// ReaderDeviceID is how NovelCheck's own reader signs the progress it writes for KOReader, so the reader never
// offers its own place back as "your e-reader is at…".
const ReaderDeviceID = "novelcheck-reader"

// KOReaderPositionOf is the newest place KOReader's progress sync has for this person in this book: the
// percentage (0 to 1), the device and when (ok false when there is none).
func (s *Store) KOReaderPositionOf(userID, bookID int64) (KosyncProgress, bool) {
	var p KosyncProgress
	err := s.DB.Get(&p, `SELECT p.document, p.progress, p.percentage, p.device, p.device_id, p.updated_at
		FROM kosync_progress p JOIN kosync_docs d ON d.document = p.document
		WHERE p.user_id = ? AND d.book_id = ? AND p.device_id != ? ORDER BY p.updated_at DESC LIMIT 1`, userID, bookID, ReaderDeviceID)
	return p, err == nil
}
