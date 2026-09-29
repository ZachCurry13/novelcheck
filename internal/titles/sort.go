package titles

import (
	"strings"
	"unicode"
)

// SortTitle is how a title sorts: without a leading "The", "A" or "An" (or
// quotes), lower case. "The Hobbit" sorts as "hobbit".
func SortTitle(title string) string {
	t := strings.TrimLeftFunc(strings.TrimSpace(title), func(r rune) bool {
		return unicode.IsPunct(r) && r != '#'
	})
	low := strings.ToLower(t)
	for _, a := range []string{"the ", "an ", "a "} {
		if strings.HasPrefix(low, a) && len(low) > len(a) {
			return strings.TrimSpace(low[len(a):])
		}
	}
	return low
}

// Name particles that belong to the last name ("Le Guin", "van Gogh").
var particles = map[string]bool{"de": true, "da": true, "del": true, "della": true, "der": true, "di": true, "du": true,
	"la": true, "le": true, "van": true, "von": true, "st.": true, "st": true, "mac": false}

// Name endings that aren't the last name.
var suffixes = map[string]bool{"jr": true, "jr.": true, "sr": true, "sr.": true, "ii": true, "iii": true, "iv": true, "phd": true}

// SortAuthor is how an author sorts: last name first, lower case, for the
// first author listed. "Terry Pratchett & Neil Gaiman" sorts as
// "pratchett, terry"; "Ursula K. Le Guin" as "le guin, ursula k.". Names
// already written "Last, First" stay as they are.
func SortAuthor(author string) string {
	a := strings.TrimSpace(author)
	for _, sep := range []string{" & ", ";", " and ", " with ", "|"} {
		if i := strings.Index(strings.ToLower(a), sep); i > 0 {
			a = a[:i]
		}
	}
	a = strings.TrimSpace(a)
	if strings.Count(a, ",") == 1 && !suffixAfterComma(a) {
		return strings.ToLower(a) // already "Last, First"
	}
	a = strings.TrimSpace(strings.ReplaceAll(a, ",", " "))
	words := strings.Fields(a)
	if len(words) < 2 {
		return strings.ToLower(a)
	}
	end := len(words)
	for end > 1 && suffixes[strings.ToLower(words[end-1])] {
		end--
	}
	start := end - 1
	for start > 1 && particles[strings.ToLower(words[start-1])] {
		start--
	}
	last := strings.Join(words[start:end], " ")
	first := strings.Join(append(append([]string{}, words[:start]...), words[end:]...), " ")
	return strings.ToLower(last + ", " + first)
}

// suffixAfterComma is true for "Martin Luther King, Jr." (not "Last, First").
func suffixAfterComma(a string) bool {
	_, after, _ := strings.Cut(a, ",")
	return suffixes[strings.ToLower(strings.TrimSpace(after))]
}
