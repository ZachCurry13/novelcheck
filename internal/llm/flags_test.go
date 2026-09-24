package llm_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestCustomFlagsInPromptAndAnswer(t *testing.T) {
	flags := []store.CustomFlag{{Key: "swearing", Label: "Heavy swearing", Description: "Frequent strong\nprofanity"}}
	p := llm.SystemPromptFor("", flags)
	if !strings.Contains(p, "- swearing (Heavy swearing): Frequent strong profanity") || !strings.Contains(p, `"custom_flags": {"swearing": true | false}`) {
		t.Fatalf("custom filters missing from prompt:\n%s", p)
	}
	if strings.Contains(llm.SystemPromptFor("", nil), "custom_flags") {
		t.Fatal("no filters, no custom_flags section")
	}
	for in, want := range map[string][]string{
		`{"spice_level": 1, "custom_flags": {"swearing": true, "gore": false}}`:  {"swearing"},
		`{"spice_level": 1, "custom_flags": {"swearing": "true", "gore": "no"}}`: {"swearing"},
		`{"spice_level": 1, "custom_flags": ["gore"]}`:                           {"gore"},
		`{"spice_level": 1, "custom_flags": "none"}`:                             nil,
		`{"spice_level": 1}`: nil,
	} {
		v, err := llm.ParseVerdict(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got := v.ToAnalysis("m").CustomFlags; !slices.Equal(got, want) {
			t.Errorf("%s: got %v, want %v", in, got, want)
		}
	}
}
