package api

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Autopilot applies only what a person would call sure: matched by ISBN or the exact title, and not genres, series numbers,
// covers or a guessed title.
func TestWhatAutopilotMayApply(t *testing.T) {
	for _, c := range []struct {
		field, how string
		want       bool
	}{
		{"identifiers", "ISBN", true}, {"series", "ISBN", true}, {"comments", "exact title and author", true}, {"title", "ISBN", true},
		{"title", "title with a typo, matched to the author's book", false}, {"tags", "ISBN", false}, {"series_index", "ISBN", false}, {"cover", "ISBN", false},
	} {
		if got := sureSuggestion(store.HCSuggestion{Field: c.field, Confidence: c.how}); got != c.want {
			t.Errorf("%s by %q: %v", c.field, c.how, got)
		}
	}
}
