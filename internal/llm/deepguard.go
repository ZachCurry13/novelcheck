package llm

// Guards against the usual mistakes of small models reading a book part:
// saying there's no romance yet giving peppers, and pasting the pepper
// scale's own wording back as if it were in the book.

import (
	"regexp"
	"strings"
)

var (
	wordRE4    = regexp.MustCompile(`\p{L}{4,}`)
	scaleWords []map[string]bool // the words of each pepper level's description
)

func init() {
	for _, line := range strings.Split(PepperLevels, "\n") {
		scaleWords = append(scaleWords, wordSet(line))
	}
}

func wordSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range wordRE4.FindAllString(strings.ToLower(s), -1) {
		m[w] = true
	}
	return m
}

// noRomance reports whether the AI's description of the romance in a part
// says there is none ("none", "No romantic or sexual content in this part"),
// as opposed to "No explicit scenes, but they kiss".
func noRomance(s string) bool {
	t := strings.Trim(strings.ToLower(strings.TrimSpace(s)), ".!\"' ")
	switch t {
	case "", "none", "no", "n/a", "na", "nothing", "null":
		return true
	}
	negative := strings.HasPrefix(t, "none") || strings.HasPrefix(t, "no ") || strings.HasPrefix(t, "there is no") || strings.HasPrefix(t, "there are no")
	return negative && !strings.Contains(t, " but ") && !strings.Contains(t, ";") && !strings.Contains(t, ", ")
}

// copiesScale reports whether s is mostly the wording of one of the pepper
// level descriptions rather than a description of the book.
func copiesScale(s string) bool {
	words := wordSet(s)
	if len(words) < 4 {
		return false
	}
	for _, level := range scaleWords {
		shared := 0
		for w := range words {
			if level[w] {
				shared++
			}
		}
		if shared*10 >= len(words)*7 {
			return true
		}
	}
	return false
}
