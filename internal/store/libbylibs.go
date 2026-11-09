package store

import "time"

// LibbyLib is one of the libraries a person borrows from.
type LibbyLib struct {
	Key  string `db:"key" json:"key"`
	Name string `db:"name" json:"name"`
}

// LibbyLibraries lists a person's libraries in the order they added them. A library saved before there
// could be several (in the user's own row) counts as the first.
func (s *Store) LibbyLibraries(userID int64) []LibbyLib {
	out := []LibbyLib{}
	_ = s.DB.Select(&out, `SELECT key, name FROM user_libby WHERE user_id = ? ORDER BY position, key`, userID)
	if len(out) == 0 {
		var row struct {
			Key  string `db:"libby_library"`
			Name string `db:"libby_name"`
		}
		if s.DB.Get(&row, `SELECT libby_library, libby_name FROM users WHERE id = ?`, userID) == nil && row.Key != "" {
			out = append(out, LibbyLib{Key: row.Key, Name: row.Name})
		}
	}
	return out
}

// AddLibbyLibrary adds (or renames) a library.
func (s *Store) AddLibbyLibrary(userID int64, key, name string) error {
	// A library from before several were possible joins the list first, so it is not lost.
	for _, l := range s.LibbyLibraries(userID) {
		if _, err := s.DB.Exec(`INSERT OR IGNORE INTO user_libby (user_id, key, name, position) VALUES (?, ?, ?, ?)`, userID, l.Key, l.Name, 0); err != nil {
			return err
		}
	}
	var next int
	_ = s.DB.Get(&next, `SELECT COALESCE(MAX(position), 0) + 1 FROM user_libby WHERE user_id = ?`, userID)
	_, err := s.DB.Exec(`INSERT INTO user_libby (user_id, key, name, position) VALUES (?, ?, ?, ?)
		ON CONFLICT (user_id, key) DO UPDATE SET name = excluded.name`, userID, key, name, next)
	if err == nil {
		s.syncFirstLibby(userID)
	}
	return err
}

// RemoveLibbyLibrary removes one library.
func (s *Store) RemoveLibbyLibrary(userID int64, key string) error {
	for _, l := range s.LibbyLibraries(userID) { // make sure an old single library is in the table before one is removed
		_, _ = s.DB.Exec(`INSERT OR IGNORE INTO user_libby (user_id, key, name, position) VALUES (?, ?, ?, 0)`, userID, l.Key, l.Name)
	}
	_, err := s.DB.Exec(`DELETE FROM user_libby WHERE user_id = ? AND key = ?`, userID, key)
	if err == nil {
		s.syncFirstLibby(userID)
		if len(s.LibbyLibraries(userID)) == 0 { // nothing left to check against: what was kept goes
			_, _ = s.DB.Exec(`DELETE FROM libby_audio WHERE user_id = ?`, userID)
		}
	}
	return err
}

// ClearLibbyLibraries removes them all.
func (s *Store) ClearLibbyLibraries(userID int64) error {
	_, _ = s.DB.Exec(`DELETE FROM user_libby WHERE user_id = ?`, userID)
	_, _ = s.DB.Exec(`DELETE FROM libby_audio WHERE user_id = ?`, userID)
	return s.SetLibbyLibrary(userID, "", "")
}

// syncFirstLibby keeps the old single-library columns equal to the first library (older code reads them).
func (s *Store) syncFirstLibby(userID int64) {
	var first LibbyLib
	_ = s.DB.Get(&first, `SELECT key, name FROM user_libby WHERE user_id = ? ORDER BY position, key LIMIT 1`, userID)
	_ = s.SetLibbyLibrary(userID, first.Key, first.Name)
}

// LibbyAudioRow is what the libraries said about a book's audiobook, for one person.
type LibbyAudioRow struct {
	BookID    int64  `db:"book_id" json:"book_id"`
	Found     bool   `db:"found" json:"found"`
	Available bool   `db:"available" json:"available"`
	Holds     int    `db:"holds" json:"holds"`
	Copies    int    `db:"copies" json:"copies"`
	WaitDays  int    `db:"wait_days" json:"wait_days"`
	URL       string `db:"url" json:"url"`
	Library   string `db:"library" json:"library"`
	CheckedAt int64  `db:"checked_at" json:"-"`
}

// SaveLibbyAudio keeps what was found for one book.
func (s *Store) SaveLibbyAudio(userID int64, r LibbyAudioRow) error {
	_, err := s.DB.Exec(`INSERT INTO libby_audio (user_id, book_id, found, available, holds, copies, wait_days, url, library, checked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (user_id, book_id) DO UPDATE SET found = excluded.found,
		available = excluded.available, holds = excluded.holds, copies = excluded.copies, wait_days = excluded.wait_days,
		url = excluded.url, library = excluded.library, checked_at = excluded.checked_at`,
		userID, r.BookID, r.Found, r.Available, r.Holds, r.Copies, r.WaitDays, r.URL, r.Library, time.Now().Unix())
	return err
}

// LibbyAudioFor returns what was kept for the given books (only those the libraries lend as audiobooks).
func (s *Store) LibbyAudioFor(userID int64, ids []int64) map[int64]LibbyAudioRow {
	out := map[int64]LibbyAudioRow{}
	for _, id := range ids {
		var r LibbyAudioRow
		if s.DB.Get(&r, `SELECT book_id, found, available, holds, copies, wait_days, url, library, checked_at FROM libby_audio
			WHERE user_id = ? AND book_id = ? AND found = 1`, userID, id) == nil {
			out[id] = r
		}
	}
	return out
}

// LibbyAudioDue lists library books without an audiobook that were not checked lately (found ones every
// 3 days, ones the libraries do not lend every 14), the oldest first.
func (s *Store) LibbyAudioDue(userID int64, limit int) []BriefBook {
	now := time.Now().Unix()
	out := []BriefBook{}
	_ = s.DB.Select(&out, `SELECT b.id, b.title, b.author, b.isbn FROM books b
		LEFT JOIN libby_audio la ON la.book_id = b.id AND la.user_id = ?
		WHERE `+ownedCond+` AND NOT `+discoverOnlyCond+` AND NOT EXISTS (SELECT 1 FROM catalog_books ac WHERE ac.book_id = b.id AND ac.format = 'audio')
		AND (la.book_id IS NULL OR la.checked_at < CASE WHEN la.found = 1 THEN ? ELSE ? END)
		ORDER BY COALESCE(la.checked_at, 0), b.id LIMIT ?`, userID, now-3*86400, now-14*86400, limit)
	return out
}

// UsersWithLibby lists the people who have a library set (so their books are checked).
func (s *Store) UsersWithLibby() []int64 {
	var ids []int64
	_ = s.DB.Select(&ids, `SELECT id FROM users WHERE id IN (SELECT user_id FROM user_libby) OR libby_library != ''`)
	return ids
}
