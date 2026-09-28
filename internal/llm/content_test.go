package llm_test

import (
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestContentInAnswers(t *testing.T) {
	cases := map[string]string{
		// Keys, labels and other names all land on known items; the rest is dropped.
		`{"spice_level": 1, "content": ["war", "Alcohol use", "sexual assault", "made-up", "war"]}`: "war,alcohol,sexual_violence",
		`{"spice_level": 1, "content": {"murder": true, "blood": "false"}}`:                         "murder",
		// An older answer's LGBTQ+ flag becomes the content item.
		`{"spice_level": 1, "lgbtq_content": true}`: "lgbtq",
		`{"spice_level": 1}`:                        "",
	}
	for out, want := range cases {
		v, err := llm.ParseVerdict(out)
		if err != nil {
			t.Fatalf("%s: %v", out, err)
		}
		a := v.ToAnalysis("m")
		if got := strings.Join(a.Content, ","); got != want || a.ContentSource != store.SourceAI {
			t.Errorf("%s → %q (%s), want %q", out, got, a.ContentSource, want)
		}
	}
	p, err := llm.ParsePart(`{"romance": "none", "level": 0, "content": ["gun-violence", "corpses"]}`)
	if err != nil || strings.Join(p.Content, ",") != "gun_violence,corpses" {
		t.Fatalf("part: %+v %v", p, err)
	}
}

func TestContentInPrompts(t *testing.T) {
	for name, prompt := range map[string]string{"quick": llm.SystemPromptFor("", nil), "deep": llm.DeepPartSystem(nil)} {
		if !strings.Contains(prompt, "4. Content Details") || !strings.Contains(prompt, "gun_violence (people shot or shot at)") ||
			!strings.Contains(prompt, `"content": ["key", ...]`) || strings.Contains(prompt, "lgbtq_content") {
			t.Errorf("%s prompt is missing the content items", name)
		}
	}
}
