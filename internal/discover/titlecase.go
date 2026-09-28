package discover

import (
	"strings"
	"unicode"
)

// small words stay lower case inside a title ("The Lord of the Rings").
var small = map[string]bool{"a": true, "an": true, "and": true, "as": true, "at": true, "but": true, "by": true,
	"for": true, "from": true, "in": true, "into": true, "nor": true, "of": true, "on": true, "or": true,
	"the": true, "to": true, "up": true, "vs": true, "with": true}

var roman = map[string]bool{"ii": true, "iii": true, "iv": true, "vi": true, "vii": true, "viii": true, "ix": true, "xi": true, "xii": true}

// TitleCase turns the New York Times' ALL-CAPS titles ("THE WOMAN IN ME")
// into ordinary title case ("The Woman in Me"). Titles that aren't all caps
// are left alone.
func TitleCase(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if s != strings.ToUpper(s) || strings.ToLower(s) == s {
		return s
	}
	words := strings.Split(strings.ToLower(s), " ")
	for i, w := range words {
		afterColon := i > 0 && strings.HasSuffix(words[i-1], ":")
		if i > 0 && i < len(words)-1 && small[w] && !afterColon {
			continue
		}
		words[i] = capWord(w)
	}
	return strings.Join(words, " ")
}

// capWord capitalizes a word and each part of a hyphenated one ("Twenty-One").
func capWord(w string) string {
	if roman[strings.Trim(w, ".,:;!?")] {
		return strings.ToUpper(w)
	}
	parts := strings.Split(w, "-")
	for i, p := range parts {
		r := []rune(p)
		for j, c := range r {
			if unicode.IsLetter(c) {
				r[j] = unicode.ToUpper(c)
				break
			}
		}
		parts[i] = string(r)
	}
	return strings.Join(parts, "-")
}
