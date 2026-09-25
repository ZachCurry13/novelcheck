package llm

// Suggested Reads: the AI picks the best books for one reader from a
// shortlist NovelCheck matched in the family's library, and may name a few
// books the family doesn't own.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// SuggestReader is what the AI is told about one reader.
type SuggestReader struct {
	Liked    []string // "Mort by Terry Pratchett (finished)"
	Disliked []string
	Limits   string // their content rules in plain words; "" = none
}

// SuggestCandidate is one shortlisted library book.
type SuggestCandidate struct {
	ID          int64
	Title       string
	Author      string
	Series      string // "Discworld #9", "" = none
	Description string
	Level       int // pepper level; -1 = not rated
}

// SuggestPick is the AI's choice from the shortlist.
type SuggestPick struct {
	ID     int64  `json:"id"`
	Reason string `json:"reason"`
}

// SuggestOutside is a book the family doesn't own.
type SuggestOutside struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	Reason string `json:"reason"`
}

// SuggestResult is the AI's answer.
type SuggestResult struct {
	Picks   []SuggestPick    `json:"picks"`
	Outside []SuggestOutside `json:"outside"`
}

// SuggestSystem is the instruction for picking suggestions.
func SuggestSystem(outside int, lang string) string {
	var b strings.Builder
	b.WriteString(`You recommend books to one reader in a family's home library. You get what they have read, are reading or want to read, books they said they don't want, their content limits, and a numbered shortlist of books from the family's library.
Pick up to 10 shortlist books this reader is most likely to enjoy next, best first. Prefer the next unread book of a series they're reading, then authors and themes they like. Skip anything like the books they don't want. Never pick a book outside their content limits.
For each pick write a short reason (at most 15 words) that names what it connects to, for example "Next in Discworld after Mort" or "Found-family fantasy like The House in the Cerulean Sea".`)
	if outside > 0 {
		fmt.Fprintf(&b, `
Also name up to %d well-known published books that are NOT in the shortlist and NOT among the books listed as read, that this reader would enjoy and that fit their content limits. Give the exact title and author.`, outside)
	}
	b.WriteString("\nWrite the reasons in " + LanguageRule(lang) + `.
Answer with JSON only: {"picks": [{"id": 12, "reason": "..."}], "outside": [{"title": "...", "author": "...", "reason": "..."}]}`)
	return b.String()
}

// SuggestUser describes the reader and the shortlist.
func SuggestUser(r SuggestReader, cands []SuggestCandidate) string {
	var b strings.Builder
	b.WriteString("Books this reader liked or wants to read:\n")
	for _, l := range r.Liked {
		b.WriteString("- " + l + "\n")
	}
	if len(r.Disliked) > 0 {
		b.WriteString("\nBooks they said they don't want:\n")
		for _, d := range r.Disliked {
			b.WriteString("- " + d + "\n")
		}
	}
	limits := r.Limits
	if limits == "" {
		limits = "none"
	}
	b.WriteString("\nContent limits: " + limits + "\n\nShortlist:\n")
	for _, c := range cands {
		fmt.Fprintf(&b, "%d. %s by %s", c.ID, c.Title, orUnknown(c.Author))
		if c.Series != "" {
			b.WriteString(" (" + c.Series + ")")
		}
		if c.Level >= 0 {
			fmt.Fprintf(&b, " [pepper level %d of 5]", c.Level)
		}
		if d := firstWords(c.Description, 45); d != "" {
			b.WriteString(": " + d)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// ParseSuggest reads the AI's JSON answer, keeping only shortlist ids.
func ParseSuggest(out string, shortlist map[int64]bool) (SuggestResult, error) {
	start, end := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return SuggestResult{}, errors.New("no JSON object in the answer")
	}
	var raw SuggestResult
	if err := json.Unmarshal([]byte(out[start:end+1]), &raw); err != nil {
		return SuggestResult{}, fmt.Errorf("invalid JSON: %w", err)
	}
	var res SuggestResult
	seen := map[int64]bool{}
	for _, p := range raw.Picks {
		if shortlist[p.ID] && !seen[p.ID] && len(res.Picks) < 10 {
			seen[p.ID] = true
			res.Picks = append(res.Picks, SuggestPick{p.ID, ShortReason(p.Reason)})
		}
	}
	for _, o := range raw.Outside {
		if t := strings.TrimSpace(o.Title); t != "" && len(res.Outside) < 5 {
			res.Outside = append(res.Outside, SuggestOutside{t, strings.TrimSpace(o.Author), ShortReason(o.Reason)})
		}
	}
	if len(res.Picks) == 0 && len(res.Outside) == 0 {
		return res, errors.New("the answer didn't pick any of the books")
	}
	return res, nil
}

func firstWords(s string, n int) string {
	f := strings.Fields(s)
	if len(f) > n {
		return strings.Join(f[:n], " ") + "…"
	}
	return strings.Join(f, " ")
}
