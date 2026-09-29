package store

// KOReader progress sync: each reader gets a sync code (Profile), KOReader
// logs in with their NovelCheck name and the code, and sends where it is in
// each book. NovelCheck keeps that for KOReader (to sync between devices)
// and moves the book along in Up Next: opened → Reading, the end →
// Finished (never back).

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"math/big"
	"strings"
)

// FinishedAt is how far into a book counts as finished (the rest is often
// notes, thanks and a preview of the next book).
const FinishedAt = 0.97

// KosyncCode returns someone's sync code, making one when they have none
// (or renew is set).
func (s *Store) KosyncCode(userID int64, renew bool) (string, error) {
	var code string
	if err := s.DB.Get(&code, `SELECT kosync_code FROM users WHERE id = ?`, userID); err != nil {
		return "", err
	}
	if code != "" && !renew {
		return code, nil
	}
	const chars = "abcdefghjkmnpqrstuvwxyz23456789"
	b := make([]byte, 12)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[n.Int64()]
	}
	code = string(b)
	_, err := s.DB.Exec(`UPDATE users SET kosync_code = ? WHERE id = ?`, code, userID)
	return code, err
}

// KosyncUser is the account a KOReader login belongs to: key is the MD5 of
// the sync code (KOReader sends that, not the code).
func (s *Store) KosyncUser(username, key string) (*User, bool) {
	u, err := s.UserByName(strings.TrimSpace(username))
	if err != nil {
		return nil, false
	}
	var code string
	if s.DB.Get(&code, `SELECT kosync_code FROM users WHERE id = ?`, u.ID) != nil || code == "" {
		return nil, false
	}
	sum := md5.Sum([]byte(code))
	want := hex.EncodeToString(sum[:])
	return u, subtle.ConstantTimeCompare([]byte(want), []byte(strings.ToLower(strings.TrimSpace(key)))) == 1
}

// KosyncProgress is where a reader is in a book, as KOReader sends it.
type KosyncProgress struct {
	Document   string  `db:"document" json:"document"`
	Progress   string  `db:"progress" json:"progress"`
	Percentage float64 `db:"percentage" json:"percentage"`
	Device     string  `db:"device" json:"device"`
	DeviceID   string  `db:"device_id" json:"device_id"`
	Timestamp  int64   `db:"updated_at" json:"timestamp"`
}

// SaveKosync keeps a reader's place in a document.
func (s *Store) SaveKosync(userID int64, p KosyncProgress) error {
	_, err := s.DB.Exec(`INSERT INTO kosync_progress (user_id, document, progress, percentage, device, device_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (user_id, document) DO UPDATE SET progress = excluded.progress,
		percentage = excluded.percentage, device = excluded.device, device_id = excluded.device_id, updated_at = excluded.updated_at`,
		userID, p.Document, p.Progress, p.Percentage, p.Device, p.DeviceID, p.Timestamp)
	return err
}

// Kosync returns a reader's place in a document (ok false if none).
func (s *Store) Kosync(userID int64, document string) (KosyncProgress, bool) {
	var p KosyncProgress
	err := s.DB.Get(&p, `SELECT document, progress, percentage, device, device_id, updated_at FROM kosync_progress
		WHERE user_id = ? AND document = ?`, userID, document)
	return p, err == nil
}

// DocBook is the book a KOReader fingerprint belongs to (0 if unknown).
func (s *Store) DocBook(document string) int64 {
	var id int64
	_ = s.DB.Get(&id, `SELECT book_id FROM kosync_docs WHERE document = ?`, document)
	return id
}

// SetDocBook remembers which book a fingerprint belongs to.
func (s *Store) SetDocBook(document string, bookID int64) {
	_, _ = s.DB.Exec(`INSERT OR REPLACE INTO kosync_docs (document, book_id) VALUES (?, ?)`, document, bookID)
}

// SyncFile is a file of one of a reader's books, to fingerprint.
type SyncFile struct {
	BookID int64  `db:"book_id"`
	Path   string `db:"path"`
}

// SyncFiles are the Calibre files of the books in someone's Up Next
// (their box set's, for a split box set's book), newest first.
func (s *Store) SyncFiles(userID int64, limit int) []SyncFile {
	out := []SyncFile{}
	_ = s.DB.Select(&out, `SELECT q.book_id, cb.path FROM queue_items q
		JOIN catalog_books cb ON cb.book_id = q.book_id OR cb.book_id IN (SELECT m.box_id FROM box_members m WHERE m.book_id = q.book_id)
		JOIN catalogs c ON c.id = cb.catalog_id AND c.source = 'calibre'
		WHERE q.user_id = ? AND cb.format NOT IN ('', 'list', 'paper')
		AND NOT EXISTS (SELECT 1 FROM kosync_docs k WHERE k.book_id = q.book_id)
		ORDER BY q.updated_at DESC LIMIT ?`, userID, limit)
	return out
}

// SyncReading moves a book along in someone's Up Next from what their
// e-reader says: reading, or finished. It never moves a book back, and adds
// one that isn't there. It reports whether anything changed.
func (s *Store) SyncReading(userID, bookID int64, finished bool) (bool, error) {
	status := "reading"
	if finished {
		status = "finished"
	}
	res, err := s.DB.Exec(`INSERT INTO queue_items (user_id, book_id, position, status, delivery_note)
		VALUES (?, ?, COALESCE((SELECT MAX(position) + 1 FROM queue_items WHERE user_id = ?), 0), ?, 'KOReader')
		ON CONFLICT (user_id, book_id) DO UPDATE SET status = excluded.status, updated_at = CURRENT_TIMESTAMP
		WHERE queue_items.status = 'queued' OR (queue_items.status = 'reading' AND excluded.status = 'finished')`,
		userID, bookID, userID, status)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
