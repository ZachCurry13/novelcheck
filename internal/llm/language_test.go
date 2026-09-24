package llm_test

import (
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/llm"
)

func TestLooksEnglish(t *testing.T) {
	cases := map[string]bool{
		"A sweet, clean romance with a few kisses; fine for teens.":                true,
		"Clean, whimsical adventure.":                                              true, // too short to tell
		"Una historia de amor dulce con besos, adecuada para adolescentes.":        false,
		"Eine süße Liebesgeschichte ohne explizite Szenen, gut geeignet ab zwölf.": false,
		"Une romance douce avec quelques baisers, convient aux adolescents.":       false,
		"甘い恋愛小説で、露骨な場面はありません。十代にも適しています。":                                          false,
	}
	for text, want := range cases {
		if got := llm.LooksEnglish(text); got != want {
			t.Errorf("LooksEnglish(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestSystemPromptFor(t *testing.T) {
	if p := llm.SystemPromptFor(""); !strings.Contains(p, "American English") || !strings.HasPrefix(p, llm.SystemPrompt) {
		t.Fatal("default language should be American English")
	}
	if p := llm.SystemPromptFor("Spanish"); !strings.Contains(p, "summary_verdict in Spanish") {
		t.Fatal("chosen language missing")
	}
	if !llm.ValidLanguage("English (UK)") || llm.ValidLanguage("Klingon") {
		t.Fatal("ValidLanguage")
	}
}
