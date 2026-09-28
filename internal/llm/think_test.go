package llm

import (
	"errors"
	"testing"
)

func TestStripThinking(t *testing.T) {
	cases := map[string]string{
		`{"spice_level": 1}`: `{"spice_level": 1}`,
		"<think>Maybe {\"level\": 5}? No.</think>\n{\"spice_level\": 1}": `{"spice_level": 1}`,
		// Some templates open the notes in the prompt, so only the end tag shows.
		"The blurb says {x}.</think>{\"spice_level\": 2}": `{"spice_level": 2}`,
	}
	for in, want := range cases {
		if got, err := stripThinking(in); err != nil || got != want {
			t.Errorf("%q → %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := stripThinking("<think>Let me look at the blurb {"); !errors.Is(err, ErrOnlyThinking) {
		t.Fatalf("cut-off thinking: %v", err)
	}
}
