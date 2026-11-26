package deepread

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

const askSystem = `You answer a parent's question about a book, using ONLY the notes you are given. The notes come from a rating of the book and, when present, from reading its full text part by part.
Rules:
- Answer in one to three plain sentences. Be neutral and modest: nothing explicit, no plot spoilers beyond what the question needs.
- If the notes answer it, answer it.
- If the notes do not say, begin with "The notes don't say." If you know this book well, you may add "Best guess:" and a short guess; never present a guess as fact.
- Never invent characters or events.`

// Answer answers a question about a book from what is known of it. Basis is "full read" when the answer rests on notes from a
// Deep Scan, else "ratings and summary" (a best guess, as the AI also draws on what it knows of the book).
func (r *Runner) Answer(ctx context.Context, book *store.Book, question string) (answer, basis string, err error) {
	question = strings.TrimSpace(question)
	if question == "" || len([]rune(question)) > 300 {
		return "", "", errors.New("ask a question of up to 300 characters")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Book: %s by %s\n", book.Title, orUnknown(book.Author))
	if book.SpiceLevel != nil {
		fmt.Fprintf(&b, "Spice level %d. %s\n", *book.SpiceLevel, book.SpiceReason)
	}
	if book.SummaryVerdict != "" {
		fmt.Fprintf(&b, "Verdict: %s\n", book.SummaryVerdict)
	}
	if book.Blurb != "" {
		fmt.Fprintf(&b, "Summary: %s\n", clip(book.Blurb, 1200))
	}
	if book.Content != "" {
		fmt.Fprintf(&b, "Content found: %s\n", book.Content)
	}
	if book.Nudity || book.SoloActs || book.HeavyInnuendo || book.DarkOccult || book.DemonicPresence {
		fmt.Fprintf(&b, "Flags: nudity=%v solo acts=%v heavy innuendo=%v dark occult=%v demonic=%v\n", book.Nudity, book.SoloActs, book.HeavyInnuendo, book.DarkOccult, book.DemonicPresence)
	}
	basis = "ratings and summary"
	if parts := r.Store.Digest(book.ID); len(parts) > 0 {
		basis = "full read"
		b.WriteString("Notes from reading the full text, part by part:\n")
		for _, p := range parts {
			fmt.Fprintf(&b, "- %s: %s\n", p.Label, p.Text)
		}
	}
	fmt.Fprintf(&b, "\nQuestion: %s", question)
	// the ordinary AIs, not the Deep Scan machine (scans wait for it; a person asking should not)
	out, _, err := askWith(ctx, r, r.Store.AIConfigs(), book.ID, askSystem, b.String(), func(s string) (string, error) {
		if strings.TrimSpace(s) == "" {
			return "", errors.New("empty answer")
		}
		return strings.TrimSpace(s), nil
	})
	return out, basis, err
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func orUnknown(s string) string {
	if strings.TrimSpace(s) == "" {
		return "unknown"
	}
	return s
}
