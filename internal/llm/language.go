package llm

import (
	"strings"
	"unicode"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// DefaultLanguage is used when no language is chosen.
const DefaultLanguage = "English (US)"

// Languages the admin can choose for text the AI writes (book summaries).
var Languages = []string{"English (US)", "English (UK)", "Spanish", "French", "German", "Portuguese", "Italian", "Dutch"}

// ValidLanguage reports whether lang is one of Languages.
func ValidLanguage(lang string) bool {
	for _, l := range Languages {
		if l == lang {
			return true
		}
	}
	return false
}

// languageName is how the prompt names the language.
func languageName(lang string) string {
	switch lang {
	case "", "English (US)":
		return "American English (US spelling)"
	case "English (UK)":
		return "British English (UK spelling)"
	}
	return lang
}

// SystemPromptFor adds the family's custom filters and the language rule to
// the analyzer prompt. Only the free text follows the language; JSON keys
// stay exactly as specified.
func SystemPromptFor(lang string, flags []store.CustomFlag) string {
	return SystemPrompt + customFlagsSection(flags) + "\n\nLANGUAGE: Write summary_verdict in " + languageName(lang) +
		", even when the title, author or blurb is in another language, and spice_reason in the same language." +
		" Keep every JSON key exactly as shown above."
}

// LanguageRule is the same instruction for other prompts (e.g. diagnosis).
func LanguageRule(lang string) string { return languageName(lang) }

var englishWords = map[string]bool{
	"the": true, "and": true, "a": true, "an": true, "of": true, "to": true, "is": true, "in": true,
	"for": true, "with": true, "this": true, "that": true, "it": true, "but": true, "as": true, "on": true,
	"are": true, "be": true, "no": true, "not": true, "or": true, "its": true, "has": true, "some": true,
}

// LooksEnglish is a quick check for summaries written in another language:
// several words and none of the most common English ones, or mostly
// non-Latin letters. Very short texts count as English (can't tell).
func LooksEnglish(text string) bool {
	letters, nonASCII := 0, 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			letters++
			if r > unicode.MaxASCII {
				nonASCII++
			}
		}
	}
	if letters > 0 && float64(nonASCII)/float64(letters) > 0.3 {
		return false
	}
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && r != '\'' })
	if len(words) < 6 {
		return true
	}
	for _, w := range words {
		if englishWords[w] {
			return true
		}
	}
	return false
}
