package store

import (
	"strings"
	"time"
)

// A book's identity: what its own first pages say it is (see internal/api/identify.go).

type Identity struct {
	Title  string
	Author string
	ISBN   string
	At     int64
}

// SaveIdentity keeps what was read (empty title: the pages did not say).
func (s *Store) SaveIdentity(bookID int64, title, author, isbn string) {
	_, _ = s.DB.Exec(`INSERT INTO book_identity (book_id, title, author, isbn, at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(book_id) DO UPDATE SET title = excluded.title, author = excluded.author, isbn = excluded.isbn, at = excluded.at`,
		bookID, strings.TrimSpace(title), strings.TrimSpace(author), strings.TrimSpace(isbn), time.Now().Unix())
}

// IdentityOf is what was read for the book, and whether anything was tried (an answer older than a month with no title is
// tried again).
func (s *Store) IdentityOf(bookID int64) (Identity, bool) {
	var id Identity
	if s.DB.Get(&id, `SELECT title AS "title", author AS "author", isbn AS "isbn", at AS "at" FROM book_identity WHERE book_id = ?`, bookID) != nil {
		return Identity{}, false
	}
	if id.Title == "" && time.Since(time.Unix(id.At, 0)) > 30*24*time.Hour {
		return Identity{}, false
	}
	return id, true
}

// IdentifyCandidates are library books with a title that looks like a file name or junk, or that Hardcover was asked about and
// does not know, that were not read yet, newest first. The caller keeps those that have an EPUB.
func (s *Store) IdentifyCandidates(limit int) []int64 {
	var ids []int64
	_ = s.DB.Select(&ids, `SELECT b.id FROM books b WHERE `+ownedCond+` AND NOT `+discoverOnlyCond+`
		AND NOT EXISTS (SELECT 1 FROM book_identity i WHERE i.book_id = b.id AND (i.title != '' OR i.at > ?))
		AND ((b.hc_id = 0 AND b.hc_rated_at > 0) OR b.title LIKE '%\_%' ESCAPE '\' OR LOWER(b.title) LIKE '%oceanofpdf%'
			OR LOWER(b.title) LIKE '%.epub%' OR LOWER(b.title) IN ('unknown', 'untitled', ''))
		ORDER BY b.id DESC LIMIT ?`, time.Now().Add(-30*24*time.Hour).Unix(), limit)
	return ids
}
