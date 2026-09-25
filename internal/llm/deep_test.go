package llm_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/llm"
)

// Real mistakes from a small model reading Harry Potter and 2001.
func TestParsePartGuards(t *testing.T) {
	cases := []struct {
		out   string
		level int
	}{
		{`{"romance": "none", "level": 4, "note": "Harry kills the basilisk with the fang"}`, 0},
		{`{"romance": "No romantic or sexual content in this part", "level": 4, "note": "Bowman discovers the airlock doors are open"}`, 0},
		{`{"romance": "", "level": 3, "note": "Moon-Watcher kills a warthog with a stone hammer"}`, 0},
		// Pasting the pepper scale back is not evidence.
		{`{"romance": "Romantic tension and kissing occur, including passionate kissing", "level": 4}`, 2},
		{`{"romance": "Sexual encounters occur on-page and include clear descriptions of sexual activity", "level": 4}`, 2},
		// Real romance keeps its level (the second look checks 3 and up).
		{`{"romance": "No explicit scenes, but Harry and Ginny kiss", "level": 2}`, 2},
		{`{"romance": "a married couple has sex in their cabin, described in detail", "level": 4}`, 4},
		{`{"romance": "a crush on a classmate", "level": 1}`, 1},
	}
	for _, c := range cases {
		p, err := llm.ParsePart(c.out)
		if err != nil || p.Level != c.level {
			t.Errorf("%s → level %d (%v), want %d", c.out, p.Level, err, c.level)
		}
	}
}

func TestConfirmedLevel(t *testing.T) {
	cases := []struct {
		c     llm.Confirmation
		level int
	}{
		{llm.Confirmation{SexOnPage: true, Evidence: "a couple has sex"}, 4},
		{llm.Confirmation{SexOnPage: true}, 0}, // a yes without naming it doesn't count
		{llm.Confirmation{Foreplay: true, Evidence: "heavy making out"}, 3},
		{llm.Confirmation{Kissing: true}, 2},
		{llm.Confirmation{Romance: true}, 1},
		{llm.Confirmation{}, 0},
	}
	for _, c := range cases {
		if got := llm.ConfirmedLevel(4, c.c); got != c.level {
			t.Errorf("%+v → %d, want %d", c.c, got, c.level)
		}
	}
	c, err := llm.ParseConfirm("```json\n{\"sex_on_page\": false, \"foreplay_on_page\": false, \"kissing\": false, \"romance\": false, \"evidence\": \"\"}\n```")
	if err != nil || llm.ConfirmedLevel(4, c) != 0 {
		t.Fatalf("parse: %+v %v", c, err)
	}
}
