package store

import (
	"strings"

	"github.com/zachcurry13/novelcheck/internal/titles"
)

// A second, more tolerant way to find a book in the library, used when the
// strict match fails: e-readers and stores shorten and reorder titles
// ("Hostile Hospital" for "The Hostile Hospital", "The Lost Files: Six's
// Legacy" for "Six's Legacy", "Snicket, Lemony" for "Lemony Snicket"). It only
// answers when exactly one book fits.

var junkAuthors = map[string]bool{"": true, "na": true, "n/a": true, "unknown": true, "owner": true, "anonymous": true}

// dropArticle takes a leading "The", "A" or "An" off a title.
func dropArticle(t string) string {
	for _, a := range []string{"the ", "an ", "a "} {
		if len(t) > len(a) && strings.EqualFold(t[:len(a)], a) {
			return strings.TrimSpace(t[len(a):])
		}
	}
	return t
}

// titleVariants are the ways a title might be written: whole, and (when an
// author can back it up) either side of a colon or dash.
func titleVariants(clean string, haveAuthor bool) []string {
	out := []string{clean}
	if haveAuthor {
		for _, sep := range []string{": ", " - ", " – ", " — "} {
			if a, b, ok := strings.Cut(clean, sep); ok {
				out = append(out, a, b)
			}
		}
	}
	return out
}

func (s *Store) matchLoose(title, author string) int64 {
	last := squash(lastName(author))
	haveAuthor := !junkAuthors[strings.ToLower(strings.TrimSpace(author))] && last != ""
	seen := map[int64]bool{}
	for _, v := range titleVariants(titles.Clean(title), haveAuthor) {
		base := squash(dropArticle(strings.TrimSpace(v)))
		if len(base) < 4 { // too short to trust ("It", "Up")
			continue
		}
		for _, key := range []string{base, "the" + base, "an" + base, "a" + base} {
			var ids []int64
			if haveAuthor {
				_ = s.DB.Select(&ids, `SELECT id FROM books WHERE norm_key = ? LIMIT 3`, key+"|"+last)
			} else {
				_ = s.DB.Select(&ids, `SELECT id FROM books WHERE norm_key LIKE ? LIMIT 3`, key+"|%")
			}
			for _, id := range ids {
				seen[id] = true
			}
		}
		if haveAuthor && len(base) >= 6 { // the library's title may carry a prefix the reader's lacks ("The Lost Files: Hunted")
			var ids []int64
			_ = s.DB.Select(&ids, `SELECT id FROM books WHERE norm_key LIKE ? LIMIT 3`, "%"+base+"|"+last)
			for _, id := range ids {
				seen[id] = true
			}
		}
	}
	if len(seen) != 1 {
		return 0
	}
	for id := range seen {
		return id
	}
	return 0
}
