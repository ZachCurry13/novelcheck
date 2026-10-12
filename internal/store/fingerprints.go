package store

// UnhashedFiles are Calibre files nobody has fingerprinted yet.
func (s *Store) UnhashedFiles(limit int) []SyncFile {
	out := []SyncFile{}
	_ = s.DB.Select(&out, `SELECT cb.book_id, cb.path FROM catalog_books cb
		JOIN catalogs c ON c.id = cb.catalog_id AND c.source = 'calibre'
		WHERE cb.format IN ('epub', 'pdf', 'mobi', 'azw3', 'azw', 'fb2', 'cbz', 'txt', 'djvu') AND cb.path != ''
		AND NOT EXISTS (SELECT 1 FROM file_fingerprints f WHERE f.path = cb.path)
		LIMIT ?`, limit)
	return out
}

// MarkFingerprint records a file as fingerprinted (md5 empty when it can't
// be read, so it isn't tried again and again) and remembers which book it is.
func (s *Store) MarkFingerprint(path, md5 string, bookID int64) {
	_, _ = s.DB.Exec(`INSERT OR REPLACE INTO file_fingerprints (path, md5, book_id) VALUES (?, ?, ?)`, path, md5, bookID)
	if md5 != "" {
		s.SetDocBook(md5, bookID)
	}
}

// FingerprintCounts says how many library files are fingerprinted, of how many.
func (s *Store) FingerprintCounts() (done, total int) {
	_ = s.DB.Get(&done, `SELECT COUNT(*) FROM file_fingerprints WHERE md5 != ''`)
	_ = s.DB.Get(&total, `SELECT COUNT(*) FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id AND c.source = 'calibre'
		WHERE cb.format IN ('epub', 'pdf', 'mobi', 'azw3', 'azw', 'fb2', 'cbz', 'txt', 'djvu') AND cb.path != ''`)
	return done, total
}
