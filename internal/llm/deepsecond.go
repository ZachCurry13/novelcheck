package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// VerifyOpenings counts the distinct sentence openings (from the second
// look) that really are in the text, ignoring case, punctuation and quote
// marks. An opening must have at least 3 words.
func VerifyOpenings(text string, openings []string) int {
	norm := " " + normWords(text) + " "
	seen := map[string]bool{}
	n := 0
	for _, o := range openings {
		w := normWords(o)
		if len(strings.Fields(w)) < 3 || seen[w] {
			continue
		}
		seen[w] = true
		if strings.Contains(norm, " "+w+" ") {
			n++
		}
	}
	return n
}

// normWords is lower-case words separated by single spaces ("don't" → "don t").
func normWords(s string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}), " ")
}

// A second opinion on a part that still counts as 4 or more, asked another
// way: a part keeps its level only if this agrees the sex act is described.

// DeepSecondSystem is the instruction for the second opinion.
const DeepSecondSystem = `You help a parent check one passage of a book. Decide whether it DESCRIBES a sex act, or only leads up to it, hints at it, or cuts away before it.
Kissing, touching, undressing and making out are not a described sex act. A scene that ends, jumps to a new chapter or to later, or uses a euphemism instead of describing it, is not described.
No explicit or graphic language. JSON only.`

// DeepSecondUser hands over the passage.
func DeepSecondUser(title, author, label, text string) string {
	return fmt.Sprintf(`Book: %s by %s
Part: %s

TEXT:
%s

Answer with JSON only:
{"described": true | false, "why": "a few modest words"}
- described: true only if the text itself describes the sex act, so a reader knows what the characters did beyond kissing, touching and undressing`,
		title, orUnknown(author), label, text)
}

// SecondOpinion is the answer.
type SecondOpinion struct {
	Described bool   `json:"described"`
	Why       string `json:"why"`
}

// ParseSecond reads the answer.
func ParseSecond(out string) (SecondOpinion, error) {
	start, end := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return SecondOpinion{}, errors.New("no JSON object in the answer")
	}
	var s SecondOpinion
	if err := json.Unmarshal([]byte(out[start:end+1]), &s); err != nil {
		return SecondOpinion{}, fmt.Errorf("invalid JSON: %w", err)
	}
	s.Why = ShortNote(s.Why)
	return s, nil
}
