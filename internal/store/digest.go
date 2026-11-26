package store

import "encoding/json"

// DigestPart is what a Deep Scan noted about one part of a book.
type DigestPart struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

// SaveDigest keeps the notes of a full read (replacing the last ones). Empty notes are not kept.
func (s *Store) SaveDigest(bookID int64, parts []DigestPart, model string) error {
	var keep []DigestPart
	for _, p := range parts {
		if p.Text != "" {
			keep = append(keep, p)
		}
	}
	if len(keep) == 0 {
		return nil
	}
	raw, _ := json.Marshal(keep)
	_, err := s.DB.Exec(`INSERT INTO book_digests (book_id, digest, model) VALUES (?, ?, ?)
		ON CONFLICT(book_id) DO UPDATE SET digest = excluded.digest, model = excluded.model, updated_at = CURRENT_TIMESTAMP`, bookID, string(raw), model)
	return err
}

// Digest returns what a full read noted about the book (nil when it has not been read in full since this was kept).
func (s *Store) Digest(bookID int64) []DigestPart {
	var raw string
	if s.DB.Get(&raw, `SELECT digest FROM book_digests WHERE book_id = ?`, bookID) != nil {
		return nil
	}
	var out []DigestPart
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}
