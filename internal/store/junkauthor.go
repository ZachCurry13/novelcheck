package store

import (
	"regexp"
	"strings"
)

// Amazon's Kindle sometimes puts an identifier after the author ("F. C. Yee FOLF7SLTZ3Y67TE2QPL6EVXGMLIPU6Z"). Such a book never matched
// its real title and author, so it was never rated. A token of 20 or more capital letters and digits, with at least one of each, is
// not part of a name.
var junkToken = regexp.MustCompile(`\b[A-Z0-9]{20,}\b`)

// A file extension left on the end of an author (a Kindle book's name: "F. C. Yee .kfx").
var junkExt = regexp.MustCompile(`(?i)\s*\.(kfx|azw3?|mobi|epub|pdf)\s*$`)

// StripJunkToken takes such an identifier out of an author ("" stays ""; a real name is untouched).
func StripJunkToken(author string) string {
	out := junkToken.ReplaceAllStringFunc(author, func(tok string) string {
		if strings.IndexFunc(tok, func(r rune) bool { return r >= '0' && r <= '9' }) < 0 || strings.IndexFunc(tok, func(r rune) bool { return r >= 'A' && r <= 'Z' }) < 0 {
			return tok
		}
		return ""
	})
	return strings.TrimSpace(junkExt.ReplaceAllString(strings.Join(strings.Fields(out), " "), ""))
}

// RepairJunkAuthors fixes the books that already carry such an identifier in the author: the author is cleaned, and a book that
// then is the same as another is joined with it. It returns the ids of the books it changed.
func (s *Store) RepairJunkAuthors() []int64 {
	var rows []struct {
		ID     int64  `db:"id"`
		Title  string `db:"title"`
		Author string `db:"author"`
	}
	_ = s.DB.Select(&rows, `SELECT id, title, author FROM books WHERE length(author) >= 24 OR LOWER(author) LIKE '%.kfx' OR LOWER(author) LIKE '%.azw%' OR LOWER(author) LIKE '%.mobi' OR LOWER(author) LIKE '%.epub' OR LOWER(author) LIKE '%.pdf'`)
	var fixed []int64
	for _, r := range rows {
		clean := StripJunkToken(r.Author)
		if clean == r.Author || clean == "" {
			continue
		}
		key := NormKey(r.Title, clean)
		var other int64
		if s.DB.Get(&other, `SELECT id FROM books WHERE norm_key = ? AND id != ?`, key, r.ID) == nil {
			if _, err := s.MergeBooks(other, r.ID); err == nil {
				fixed = append(fixed, other)
			}
			continue
		}
		if _, err := s.DB.Exec(`UPDATE books SET author = ?, norm_key = ? WHERE id = ?`, clean, key, r.ID); err == nil {
			fixed = append(fixed, r.ID)
		}
	}
	return fixed
}
