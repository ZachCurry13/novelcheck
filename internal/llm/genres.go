package llm

// Genres for books Calibre has no tags for: the AI sorts a batch of books
// into NovelCheck's categories (package genres) and fiction/nonfiction.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/genres"
)

// GenreBook is one book in a batch.
type GenreBook struct {
	ID                         int64
	Title, Author, Description string
}

// GenreAnswer is the AI's categories for one book.
type GenreAnswer struct {
	Keys []string
	Kind string // fiction, nonfiction or ""
}

// GenreSystem is the instruction for sorting books into categories.
func GenreSystem() string {
	var b strings.Builder
	b.WriteString("You sort books into categories. For each numbered book, pick 1 to 3 categories from this list, most fitting first, and say whether it is fiction:\n")
	for _, g := range genres.All {
		fmt.Fprintf(&b, "- %s: %s\n", g.Key, g.Label)
	}
	b.WriteString(`Use only these keys. If you don't know a book, judge from its title, author and description.
Answer with JSON only: {"books": [{"id": 12, "genres": ["fantasy", "ya"], "fiction": true}]}`)
	return b.String()
}

// GenreUser lists the books.
func GenreUser(books []GenreBook) string {
	var b strings.Builder
	for _, x := range books {
		fmt.Fprintf(&b, "%d. %s by %s", x.ID, x.Title, orUnknown(x.Author))
		if d := firstWords(x.Description, 40); d != "" {
			b.WriteString(": " + d)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// ParseGenres reads the answer, keeping only the batch's books and known keys.
func ParseGenres(out string, ids map[int64]bool) (map[int64]GenreAnswer, error) {
	start, end := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return nil, errors.New("no JSON object in the answer")
	}
	var raw struct {
		Books []struct {
			ID      int64    `json:"id"`
			Genres  []string `json:"genres"`
			Fiction *bool    `json:"fiction"`
		} `json:"books"`
	}
	if err := json.Unmarshal([]byte(out[start:end+1]), &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	res := map[int64]GenreAnswer{}
	for _, b := range raw.Books {
		if !ids[b.ID] {
			continue
		}
		var a GenreAnswer
		for _, k := range b.Genres {
			if k = strings.ToLower(strings.TrimSpace(k)); genres.Valid(k) && len(a.Keys) < 3 && !contains(a.Keys, k) {
				a.Keys = append(a.Keys, k)
			}
		}
		if b.Fiction != nil {
			a.Kind = map[bool]string{true: "fiction", false: "nonfiction"}[*b.Fiction]
		}
		res[b.ID] = a
	}
	if len(res) == 0 {
		return nil, errors.New("the answer didn't cover any of the books")
	}
	return res, nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
