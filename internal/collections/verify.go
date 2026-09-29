package collections

import (
	"context"
	"fmt"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The second look: a differently worded question on whether each book is
// really about a shelf's theme. New AI picks keep only a clear yes; checking
// an existing shelf offers to take off the clear no's.

// checkBatch is how many books one question covers (each about 110 tokens).
const checkBatch = 40

// Answer is the second look at one book: Fits is "yes", "no" or "unsure".
type Answer struct {
	Fits   string `json:"fits"`
	Reason string `json:"reason"`
}

func verifySystem(lang string) string {
	return `A parent is checking whether books belong on a themed shelf of the family library.
For each book, answer whether someone looking for the theme would be glad to find this book there because the book itself is about
the theme: its subject, its setting or its main characters. Answer "no" for a book that only shares a mood, a lesson, a genre or a word
with the theme. Answer "unsure" when the details given aren't enough to tell.
Give a short reason (under 12 words, no spoilers) in ` + lang + `.
Reply with JSON only: {"answers": [{"id": 123, "fits": "yes", "reason": "..."}]}`
}

// checkLine is one book for the second look, with more of its description
// than the pick list has.
func checkLine(b store.Book) string {
	about := strings.TrimSpace(b.Premise + " " + b.Blurb)
	if about == "" {
		about = b.Description
	}
	if r := []rune(about); len(r) > 300 {
		about = string(r[:300]) + "…"
	}
	return fmt.Sprintf("%d | %s | %s | %s | %s | %s\n", b.ID, b.Title, b.Author, b.Series,
		strings.Trim(b.Genres, ","), strings.ReplaceAll(about, "\n", " "))
}

// Verify gives the second look to books, checkBatch at a time; progress (if
// set) hears how many are done. Books the AI didn't answer are left out.
func Verify(ctx context.Context, st *store.Store, theme string, books []store.Book, progress ...func(done int)) (map[int64]Answer, error) {
	out := map[int64]Answer{}
	ais := st.AIConfigs()
	system := verifySystem(st.Setting(store.KeyLanguage))
	for start := 0; start < len(books); start += checkBatch {
		part := books[start:min(start+checkBatch, len(books))]
		var list strings.Builder
		for _, b := range part {
			list.WriteString(checkLine(b))
		}
		var raw struct {
			Answers []struct {
				ID     int64  `json:"id"`
				Fits   any    `json:"fits"` // "yes", or true from a model that ignores the format
				Reason string `json:"reason"`
			} `json:"answers"`
		}
		user := "Shelf check. Theme: " + theme + "\n\nBooks (id | title | author | series | genres | about):\n" + list.String()
		if _, err := llm.Ask(ctx, st, ais, 0, system, user, func(s string) error { return decode(s, &raw) }); err != nil {
			return out, fmt.Errorf("the AI couldn't check the books: %w", err)
		}
		offered := map[int64]bool{}
		for _, b := range part {
			offered[b.ID] = true
		}
		for _, a := range raw.Answers {
			if offered[a.ID] {
				out[a.ID] = Answer{Fits: fitsWord(a.Fits), Reason: llm.ShortReason(a.Reason)}
			}
		}
		for _, f := range progress {
			f(start + len(part))
		}
	}
	return out, nil
}

// fitsWord reads "yes"/"no"/"unsure" (or true/false) as one of the three.
func fitsWord(v any) string {
	switch x := v.(type) {
	case bool:
		return map[bool]string{true: "yes", false: "no"}[x]
	case string:
		switch w := strings.ToLower(strings.TrimSpace(x)); w {
		case "yes", "no":
			return w
		case "true":
			return "yes"
		case "false":
			return "no"
		}
	}
	return "unsure"
}

// CheckTokens estimates the tokens of checking n books, for the hourly cap.
func CheckTokens(n int) int {
	batches := (n + checkBatch - 1) / checkBatch
	return batches*600 + n*140
}
