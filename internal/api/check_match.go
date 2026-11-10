package api

import (
	"strings"
	"unicode"
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
