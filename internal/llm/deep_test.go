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
		{`{"romance": "More developed romance with stronger attraction and kissing, including passionate kissing", "level": 4}`, 2},
		{`{"romance": "Strong sexual attraction and desire are present", "level": 3}`, 2},
		{`{"romance": "Sexual encounters occur on-page and include clear descriptions of sexual activity", "level": 4}`, 2},
		// Real romance keeps its level (the second look checks 3 and up).
		{`{"romance": "No explicit scenes, but Harry and Ginny kiss", "level": 2}`, 2},
		{`{"romance": "they spend the night together, off the page", "level": 3}`, 3},
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
