package seasons

import (
	"strings"
	"unicode"
)

// A shelf's words match whole words, so "saint" finds "saint" and "saints"
// but not "saintly", and a phrase must appear word for word. A word written
// with a capital ("Lent", "Advent") matches only with its capital, so "he
// lent her a book" or "the advent of radio" stays off the shelf. The season's
// Not phrases ("Saint Louis") are skipped before matching.

// tokens splits text into words: letters and digits, apostrophes dropped
// ("saint's" is "saints"), everything else a break.
func tokens(text string) []string {
	var out []string
	var w strings.Builder
	flush := func() {
		if w.Len() > 0 {
			out = append(out, w.String())
			w.Reset()
		}
	}
	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			w.WriteRune(r)
		case r == '\'' || r == '’':
		default:
			flush()
		}
	}
	flush()
	return out
}

// same reports whether token t is word w or its plural (s, es, y → ies).
func same(t, w string) bool {
	if t == w {
		return true
	}
	if rest, ok := strings.CutPrefix(t, w); ok && (rest == "s" || rest == "es") {
		return true
	}
	stem, ok := strings.CutSuffix(w, "y")
	return ok && t == stem+"ies"
}

// at reports whether phrase starts at position i of toks.
func at(toks, phrase []string, i int) bool {
	if i+len(phrase) > len(toks) {
		return false
	}
	for k, p := range phrase {
		if toks[i+k] != p && (k < len(phrase)-1 || !same(toks[i+k], p)) {
			return false
		}
	}
	return true
}

func has(toks, phrase []string) bool {
	for i := range toks {
		if at(toks, phrase, i) {
			return true
		}
	}
	return false
}

// Matches reports whether text (a book's title, tags and description) has
// one of the season's words.
func (s Season) Matches(text string) bool {
	exact := tokens(text)
	lower := make([]string, len(exact))
	for i, t := range exact {
		lower[i] = strings.ToLower(t)
	}
	for _, n := range s.Not { // blank out the phrases that only look like a match
		not := tokens(strings.ToLower(n))
		for i := range lower {
			if at(lower, not, i) {
				for k := i; k < i+len(not); k++ {
					lower[k], exact[k] = "", ""
				}
			}
		}
	}
	for _, w := range s.Words {
		phrase := tokens(w)
		if len(phrase) == 0 {
			continue
		}
		if r := []rune(w)[0]; unicode.IsUpper(r) {
			if has(exact, phrase) {
				return true
			}
		} else if has(lower, phrase) {
			return true
		}
	}
	return false
}
