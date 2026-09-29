package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Box sets: which books an omnibus or box set edition holds, for a parent
// to check before NovelCheck splits it into its books.

// BoxContentsSystem is the instruction.
const BoxContentsSystem = `You know published books. Say which books a box set, omnibus or collected edition contains, in reading order.
List only books you are sure are in it, with the titles they were published under. If you don't know, answer an empty list. JSON only.`

// BoxContentsUser describes the box set.
func BoxContentsUser(title, author, series, description string) string {
	if r := []rune(description); len(r) > 1500 {
		description = string(r[:1500])
	}
	return fmt.Sprintf(`Box set: %s
Author: %s
Series: %s
Description: %s

Answer with JSON only:
{"books": [{"title": "...", "number": 1}, ...]}
- title: each book's own title, without the series name or number
- number: its number in the series, or 0`, title, orUnknown(author), orUnknown(series), orUnknown(description))
}

// ParseBoxContents reads the answer: at most 30 books.
func ParseBoxContents(out string) ([]store.BoxEntry, error) {
	start, end := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return nil, errors.New("no JSON object in the answer")
	}
	var v struct {
		Books []store.BoxEntry `json:"books"`
	}
	if err := json.Unmarshal([]byte(out[start:end+1]), &v); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	books := []store.BoxEntry{}
	for _, b := range v.Books {
		if t := strings.TrimSpace(b.Title); t != "" && len(books) < 30 {
			books = append(books, store.BoxEntry{Title: t, Number: b.Number})
		}
	}
	return books, nil
}
