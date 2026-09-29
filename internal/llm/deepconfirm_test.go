package llm_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/llm"
)

func TestConfirmedLevel(t *testing.T) {
	scene := "Two adults spend the night together."
	cases := []struct {
		c        llm.Confirmation
		verified int
		level    int
	}{
		{llm.Confirmation{HowFar: "detailed_sex", Evidence: "an explicit scene"}, 3, 4},
		{llm.Confirmation{HowFar: "detailed_sex", Evidence: "an explicit scene"}, 2, 3}, // the text doesn't back it up
		{llm.Confirmation{HowFar: "detailed_sex"}, 0, 3},
		{llm.Confirmation{HowFar: "brief_sex", Scene: scene}, 5, 3},
		{llm.Confirmation{HowFar: "implied_sex", Evidence: "a night together, off the page"}, 0, 3},
		{llm.Confirmation{HowFar: "sexual_touching", Scene: scene}, 0, 3},
		{llm.Confirmation{HowFar: "making_out"}, 0, 2}, // desire has to be named
		{llm.Confirmation{HowFar: "kissing"}, 0, 2},
		{llm.Confirmation{HowFar: "romance"}, 0, 1},
		{llm.Confirmation{HowFar: "none"}, 0, 0},
		{llm.Confirmation{HowFar: "something else"}, 0, 0},
	}
	for _, c := range cases {
		if got := llm.ConfirmedLevel(4, c.c, c.verified); got != c.level {
			t.Errorf("%+v (%d verified) → %d, want %d", c.c, c.verified, got, c.level)
		}
	}
	c, err := llm.ParseConfirm("```json\n" + `{"how_far": "Implied_Sex", "sex_sentences": [], "evidence": "a night off the page", "scene": "They go to bed; the chapter ends."}` + "\n```")
	if err != nil || c.HowFar != "implied_sex" || llm.ConfirmedLevel(5, c, 0) != 3 || c.Scene == "" {
		t.Fatalf("parse: %+v %v", c, err)
	}
	if s, err := llm.ParseSecond(`{"described": false, "why": "it cuts away"}`); err != nil || s.Described {
		t.Fatalf("second: %+v %v", s, err)
	}
}

// The openings must really be in the text: a made-up sentence doesn't count,
// nor a repeat, nor one too short to tell.
func TestVerifyOpenings(t *testing.T) {
	text := "The rain stopped at dawn. “We shouldn’t,” she said, and he agreed.\n\nThey walked down to the harbor together. Gulls circled the boats."
	got := llm.VerifyOpenings(text, []string{
		"The rain stopped at",      // yes
		`"We shouldn't," she said`, // yes, whatever the quote marks
		"They walked down to",      // yes
		"They walked down to",      // a repeat
		"Gulls circled",            // too short
		"She kissed him slowly",    // not in the text
	})
	if got != 3 {
		t.Fatalf("verified %d, want 3", got)
	}
}
