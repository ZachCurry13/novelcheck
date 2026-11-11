package api

import (
	"context"
	"strings"
	"unicode"

	"github.com/zachcurry13/novelcheck/internal/enrich"
)

// Check a book: a book found by its title alone must not be someone else's book of the same name
// ("Part of Your World" by Liz Braswell is not the one by Abby Jimenez).

// otherAuthor says whether book id is clearly by someone other than author. With no author on either
// side it can't tell, and says no.
func (s *Server) otherAuthor(id int64, author string) bool {
	if id == 0 || strings.TrimSpace(author) == "" {
		return false
	}
	b, err := s.Store.BookByID(id, nil)
	if err != nil || strings.TrimSpace(b.Author) == "" {
		return false
	}
	return !sameWriter(b.Author, author)
}

// sameWriter: a last name of one appears among the names of the other ("Rowling, J. K." and "J.K. Rowling";
// "A & B" and "B"). First names alone don't count.
func sameWriter(a, b string) bool {
	return anyIn(lastNames(a), nameWords(b)) || anyIn(lastNames(b), nameWords(a))
}

func anyIn(names []string, words map[string]bool) bool {
	for _, n := range names {
		if words[n] {
			return true
		}
	}
	return false
}

func nameWords(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if len(w) >= 3 {
			out[w] = true
		}
	}
	return out
}

// lastNames is the last name of each author in "First Last", "Last, First", "A & B", "A; B" and "A and B".
func lastNames(s string) []string {
	s = strings.NewReplacer(" & ", ";", " and ", ";", " And ", ";").Replace(s)
	var out []string
	for _, one := range strings.Split(s, ";") {
		if i := strings.Index(one, ","); i > 0 {
			one = one[:i]
		}
		words := strings.FieldsFunc(strings.ToLower(one), func(r rune) bool { return !unicode.IsLetter(r) })
		for i := len(words) - 1; i >= 0; i-- {
			if len(words[i]) >= 3 && words[i] != "jr" {
				out = append(out, words[i])
				break
			}
		}
	}
	return out
}

// realBook says whether what an AI read off a photo is a book that exists under that title by that author:
// one the family already has, or one the book catalogue finds. It is how a misread cover is caught.
func (s *Server) realBook(ctx context.Context, f enrich.Found) bool {
	if id := s.Store.MatchBook(f.Title, f.Author); id != 0 && !s.otherAuthor(id, f.Author) {
		return true
	}
	c, ok := s.Worker.Enricher().FindTitle(ctx, f.Title, f.Author)
	if !ok || !titlesAgree(c.Title, f.Title) {
		return false
	}
	return f.Author == "" || c.Author == "" || sameWriter(c.Author, f.Author)
}

// titlesAgree: one title holds the other once case, spaces and punctuation are gone ("Fourth Wing" and
// "Fourth Wing (The Empyrean, #1)"); a title of under four letters has to match whole.
func titlesAgree(a, b string) bool {
	squash := func(s string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return unicode.ToLower(r)
			}
			return -1
		}, s)
	}
	x, y := squash(a), squash(b)
	if x == "" || y == "" {
		return false
	}
	if len(x) < 4 || len(y) < 4 {
		return x == y
	}
	return strings.Contains(x, y) || strings.Contains(y, x)
}
