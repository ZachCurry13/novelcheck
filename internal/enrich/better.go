package enrich

import "strings"

// BetterName says whether a book's catalog entry (Open Library) is its true
// title and author, written properly, for a book NovelCheck has by a typed,
// read or copied name ("Abby jiminez a married little Christmas" is "A
// Married Little Christmas" by Abby Jimenez). It must be the same book:
// found by the book's own ISBN, or every word of the catalog title is in the
// name NovelCheck has; and the author agrees (the last name, allowing a typo
// or two, in the author or the name NovelCheck has). It gives the name to
// use and whether it differs from what NovelCheck has.
func BetterName(title, author string, byISBN bool, f Found) (Found, bool) {
	f.Title, f.Author = strings.TrimSpace(f.Title), strings.TrimSpace(f.Author)
	if f.Title == "" || f.Author == "" {
		return Found{}, false
	}
	if !byISBN {
		have := map[string]bool{}
		for _, w := range words(title) {
			have[w] = true
		}
		ws := words(f.Title)
		if len(ws) == 0 || strings.TrimSpace(author) == "" {
			return Found{}, false // a bare title with no author could be any book
		}
		for _, w := range ws {
			if !have[w] {
				return Found{}, false
			}
		}
	}
	if !sameAuthor(f.Author, author+" "+title) {
		return Found{}, false
	}
	if f.Title == strings.TrimSpace(title) && f.Author == strings.TrimSpace(author) {
		return f, false
	}
	return f, true
}

// sameAuthor: the catalog author's last name, or a word within two letters
// of it ("jiminez" for "Jimenez"), is in text.
func sameAuthor(catalog, text string) bool {
	parts := strings.Fields(strings.ToLower(catalog))
	if len(parts) == 0 {
		return false
	}
	last := squash(parts[len(parts)-1])
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return r == ' ' || r == ',' || r == '.' || r == '-' || r == '_' }) {
		w = squash(w)
		if w == last || (len(last) >= 5 && editDistance(w, last) <= 2) {
			return true
		}
	}
	return false
}

func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}
