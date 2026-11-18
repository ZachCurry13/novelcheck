package store

import (
	"regexp"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/titles"
)

// Telling that two files on an e-reader are the same book when their names differ: "Fall of Five, The - Pittacus
// Lore (4006).epub" (Calibre's own naming), "Pittacus Lore - The Fall of Five.epub" and "The-Fall-of-Five.epub" are
// one book. Every reading of a name is tried, the Calibre number and the library-style ", The" are undone, and a
// reading whose "title" is really an author's name is left out (otherwise all of an author's books would look alike).

var (
	calibreNumber = regexp.MustCompile(`\s*\(\d+\)\s*$`)
	sortedArticle = regexp.MustCompile(`(?i)^(.+),\s*(the|a|an)$`)
)

// TidyBookTitle undoes file-name habits: "_" and "-" between words, a trailing Calibre number "(4006)", and the
// library-style "Fall of Five, The".
func TidyBookTitle(t string) string {
	t = strings.TrimSpace(strings.NewReplacer("_", " ").Replace(t))
	if !strings.Contains(t, " ") && strings.Count(t, "-") >= 2 { // The-Fall-of-Five
		t = strings.ReplaceAll(t, "-", " ")
	}
	t = calibreNumber.ReplaceAllString(t, "")
	if m := sortedArticle.FindStringSubmatch(t); m != nil {
		t = m[2] + " " + m[1]
	}
	return strings.TrimSpace(t)
}

// AuthorNames is every author the library knows, squashed ("pittacuslore", and "lorepittacus" for "Lore, Pittacus").
func (s *Store) AuthorNames() map[string]bool {
	var names []string
	_ = s.DB.Select(&names, `SELECT DISTINCT author FROM books WHERE author != ''`)
	out := map[string]bool{}
	for _, a := range names {
		for _, one := range strings.FieldsFunc(a, func(r rune) bool { return r == '&' || r == ';' }) {
			out[squash(one)] = true
			if last, first, ok := strings.Cut(one, ","); ok {
				out[squash(first+" "+last)] = true
			}
		}
	}
	return out
}

// DeviceKeys gives a file on an e-reader its library book (0 when none) and the title keys it can be grouped by.
func (s *Store) DeviceKeys(filename, title, author string, authors map[string]bool) (bookID int64, keys []string) {
	type reading struct{ t, a string }
	var rs []reading
	if strings.TrimSpace(title) != "" {
		rs = append(rs, reading{title, author})
	}
	for _, g := range titles.FromFilename(filename) {
		rs = append(rs, reading{g.Title, g.Author})
	}
	seen := map[string]bool{}
	for _, r := range rs {
		t := TidyBookTitle(r.t)
		a := calibreNumber.ReplaceAllString(strings.TrimSpace(r.a), "")
		if t == "" || authors[squash(t)] {
			continue // empty, or an author's name read as a title
		}
		if k := squash(titles.Clean(t)); k != "" && !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
		if bookID == 0 {
			bookID = s.MatchBook(t, a)
		}
	}
	return bookID, keys
}
