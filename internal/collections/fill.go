// Package collections fills collections with the AI: a parent describes a
// theme ("dragon adventures for ages 8-10") and the AI picks matching books
// from the family's libraries, with a reason each; about once a week it also
// proposes a few themed collections of its own for a parent to keep or drop.
package collections

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/genres"
	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// maxPicks is the most books one AI fill proposes; maxCandidates how many
// the AI chooses from (each costs about 60 tokens).
const (
	maxPicks      = 40
	maxCandidates = 150
	estTokens     = 13000 // plan + pick, for the hourly cap
)

// Pick is a book the AI chose, with its reason.
type Pick struct {
	Book   store.Book `json:"book"`
	Reason string     `json:"reason"`
}

// plan is how the AI turns a theme into a search of the library.
type plan struct {
	Words  []string `json:"words"`
	Genres []string `json:"genres"`
	Kind   string   `json:"kind"`
}

// ErrBudget means the hourly token limit would be passed.
var ErrBudget = errors.New("the hourly AI limit is reached; try again in a little while")

// Fill asks the AI for the family's books that fit theme, skipping exclude.
func Fill(ctx context.Context, st *store.Store, theme string, exclude []int64) ([]Pick, error) {
	theme = strings.TrimSpace(theme)
	if theme == "" {
		return nil, errors.New("describe the collection first")
	}
	if !withinBudget(st) {
		return nil, ErrBudget
	}
	ais := st.AIConfigs()
	var p plan
	if _, err := llm.Ask(ctx, st, ais, 0, planSystem(), "Theme: "+theme, func(out string) error {
		return decode(out, &p)
	}); err != nil {
		return nil, fmt.Errorf("the AI couldn't plan the search: %w", err)
	}
	books, err := st.ThemeCandidates(p.Words, p.Genres, p.Kind, exclude, maxCandidates)
	if err != nil {
		return nil, err
	}
	if len(books) == 0 {
		return []Pick{}, nil
	}
	byID := map[int64]store.Book{}
	var list strings.Builder
	for _, b := range books {
		byID[b.ID] = b
		list.WriteString(describe(b))
	}
	var raw struct {
		Picks []struct {
			ID     int64  `json:"id"`
			Reason string `json:"reason"`
		} `json:"picks"`
	}
	user := fmt.Sprintf("Theme: %s\n\nBooks (id | title | author | series | genres | peppers | about):\n%s", theme, list.String())
	if _, err := llm.Ask(ctx, st, ais, 0, pickSystem(st.Setting(store.KeyLanguage)), user, func(out string) error {
		return decode(out, &raw)
	}); err != nil {
		return nil, fmt.Errorf("the AI couldn't pick the books: %w", err)
	}
	out := []Pick{}
	seen := map[int64]bool{}
	for _, r := range raw.Picks {
		if b, ok := byID[r.ID]; ok && !seen[r.ID] && len(out) < maxPicks {
			seen[r.ID] = true
			out = append(out, Pick{Book: b, Reason: llm.ShortReason(r.Reason)})
		}
	}
	return out, nil
}

func planSystem() string {
	keys := make([]string, 0, len(genres.All))
	for _, g := range genres.All {
		keys = append(keys, g.Key)
	}
	return `You help a family find books for a themed collection in their own library.
Turn the theme into a search. Reply with JSON only:
{"words": ["up to 15 lowercase words or short phrases likely to appear in the titles, tags or descriptions of fitting books"],
 "genres": ["genre keys that fit, from: ` + strings.Join(keys, ", ") + `"],
 "kind": "fiction" or "nonfiction" or ""}`
}

func pickSystem(lang string) string {
	return `You pick books for a family's themed collection from the list given. Choose the books that truly fit the theme
(up to ` + strconv.Itoa(maxPicks) + `, best first). Leave out books that only share a word with it. Respect ages or limits the theme mentions.
Give each a short reason (under 15 words, no spoilers) in ` + lang + `. Use only ids from the list.
Reply with JSON only: {"picks": [{"id": 123, "reason": "..."}]}`
}

// describe is one candidate line for the AI.
func describe(b store.Book) string {
	about := b.Premise
	if about == "" {
		about = b.Blurb
	}
	if about == "" {
		about = b.Description
	}
	if r := []rune(about); len(r) > 150 {
		about = string(r[:150]) + "…"
	}
	peppers := "?"
	if b.SpiceLevel != nil {
		peppers = strconv.Itoa(*b.SpiceLevel)
	}
	series := b.Series
	if series != "" && b.SeriesIndex > 0 {
		series += fmt.Sprintf(" #%g", b.SeriesIndex)
	}
	return fmt.Sprintf("%d | %s | %s | %s | %s | %s | %s\n", b.ID, b.Title, b.Author, series,
		strings.Trim(b.Genres, ","), peppers, strings.ReplaceAll(about, "\n", " "))
}

// decode reads the JSON object in an AI answer.
func decode(out string, v any) error {
	start, end := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return errors.New("no JSON object in the answer")
	}
	return json.Unmarshal([]byte(out[start:end+1]), v)
}

// withinBudget: collection work never pushes past the hourly token cap.
func withinBudget(st *store.Store) bool {
	limit := st.SettingInt(store.KeyTokensPerHour)
	used, err := st.TokensSince("-1 hour")
	return limit <= 0 || (err == nil && used+estTokens <= limit)
}
