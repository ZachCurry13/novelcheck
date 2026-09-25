package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// PartResult is what the AI found in one part of a book (Deep read).
type PartResult struct {
	Level           int
	Evidence        string // the romantic or sexual content on the page, in the AI's words
	Note            string
	Nudity          bool
	SoloActs        bool
	HeavyInnuendo   bool
	PlayfulFantasy  bool
	DarkOccult      bool
	DemonicPresence bool
	LGBTQ           bool
	CustomFlags     []string
}

const deepPartPrompt = `You are reading one part of a book for parents who want to know about its romantic/sexual content and certain themes. Judge only the text you are given.

STRICT RULES:
1. NO EXPLICIT/GRAPHIC LANGUAGE: describe things modestly (e.g., "an explicit scene", "solo acts", "heavy innuendo").
2. NO MAJOR SPOILERS in the note.
3. JSON ONLY.

1. Pepper levels (0-5). Give the highest level that occurs in THIS PART:
` + PepperLevels + `

WHAT PEPPERS MEASURE: ONLY romantic and sexual content. Violence, fighting, killing, death, injury, monsters, danger, suspense, fear, horror, magic, science, religion, politics and every other mature theme are NOT romance: a part without romance or sexual content is level 0, however dark, violent, tense or frightening it is.
First write in "romance" what romantic or sexual content actually happens on the page in THIS text, in your own words (e.g. "two characters kiss", "a couple has sex, described in detail"), or "none". Don't copy the level descriptions above. Then give the level that matches it. "none" means level 0.
Level 3 or higher needs sexual content happening on the page in this text; if "romance" doesn't describe any, use level 2 or lower.

` + ContentGuide + `
Only mark nudity, solo_acts or heavy_innuendo for sexual content on the page in this text. Mark playful_fantasy only for actual magic or fantasy creatures in this text.

OUTPUT FORMAT (JSON ONLY, in this order):
{"romance": "what romantic or sexual content happens on the page in this text, modestly, or \"none\"", "level": 0 | 1 | 2 | 3 | 4 | 5, "note": "one short, modest sentence on any romance/sexual content or flagged themes in this part, or \"\" if there is none", "nudity": true | false, "solo_acts": true | false, "heavy_innuendo": true | false, "playful_fantasy": true | false, "dark_occult": true | false, "demonic_presence": true | false, "lgbtq_content": true | false}`

// DeepPartSystem is the instruction for reading one part, with the family's
// custom filters.
func DeepPartSystem(flags []store.CustomFlag) string {
	return deepPartPrompt + customFlagsSection(flags)
}

// DeepPartUser hands the AI one part of the book.
func DeepPartUser(title, author, label string, n, total int, text string) string {
	return fmt.Sprintf("Book: %s by %s\nPart %d of %d (%s)\n\nTEXT:\n%s", title, orUnknown(author), n, total, label, text)
}

// ParsePart reads the JSON answer for one part.
func ParsePart(out string) (PartResult, error) {
	start, end := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return PartResult{}, errors.New("no JSON object in the answer")
	}
	var raw struct {
		Level           json.RawMessage `json:"level"`
		Romance         string          `json:"romance"`
		Note            string          `json:"note"`
		Nudity          bool            `json:"nudity"`
		SoloActs        bool            `json:"solo_acts"`
		HeavyInnuendo   bool            `json:"heavy_innuendo"`
		PlayfulFantasy  bool            `json:"playful_fantasy"`
		DarkOccult      bool            `json:"dark_occult"`
		DemonicPresence bool            `json:"demonic_presence"`
		LGBTQ           bool            `json:"lgbtq_content"`
		Custom          json.RawMessage `json:"custom_flags"`
	}
	if err := json.Unmarshal([]byte(out[start:end+1]), &raw); err != nil {
		return PartResult{}, fmt.Errorf("invalid JSON: %w", err)
	}
	level, ok := spiceFrom(raw.Level)
	if !ok {
		return PartResult{}, fmt.Errorf("level must be 0-5, got %s", raw.Level)
	}
	evidence := ShortNote(raw.Romance)
	switch {
	case noRomance(evidence):
		level, evidence = 0, "" // however intense the part, without romance it has no peppers
	case level >= 3 && copiesScale(evidence):
		level = 2 // it repeated a level's description instead of saying what's in the book
	}
	return PartResult{Level: level, Evidence: evidence, Note: ShortNote(raw.Note), Nudity: raw.Nudity, SoloActs: raw.SoloActs,
		HeavyInnuendo: raw.HeavyInnuendo, PlayfulFantasy: raw.PlayfulFantasy, DarkOccult: raw.DarkOccult || raw.DemonicPresence,
		DemonicPresence: raw.DemonicPresence, LGBTQ: raw.LGBTQ, CustomFlags: customFlagsFrom(raw.Custom)}, nil
}

// ShortNote keeps a part note to one tidy line of at most 200 characters.
func ShortNote(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 200 {
		s = strings.TrimSpace(string(r[:199])) + "…"
	}
	return s
}

// DeepWrapUpSystem asks for the final reason and summary from the notes.
const DeepWrapUpSystem = `You write short, modest notes for parents about a book that was read in full. No explicit or graphic language, no major spoilers. JSON only.`

// DeepWrapUpUser lists what each part contained.
func DeepWrapUpUser(title, author string, level int, notes []string, lang string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Book: %s by %s\nHighest pepper level found in the text: %d (0-5 scale).\nWhat the parts contained:\n", title, orUnknown(author), level)
	for _, n := range notes {
		b.WriteString("- " + n + "\n")
	}
	b.WriteString(`Reply with JSON only: {"spice_reason": "3-8 words naming what sets the level, e.g. \"Explicit scene in chapter 12\"", "summary_verdict": "1-2 sentence recommendation for parents"}. Write both in ` + languageName(lang) + ".")
	return b.String()
}

// ParseWrapUp reads the final reason and summary.
func ParseWrapUp(out string) (reason, summary string, ok bool) {
	start, end := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return "", "", false
	}
	var v struct {
		Reason  string `json:"spice_reason"`
		Summary string `json:"summary_verdict"`
	}
	if json.Unmarshal([]byte(out[start:end+1]), &v) != nil {
		return "", "", false
	}
	return ShortReason(v.Reason), strings.TrimSpace(v.Summary), v.Summary != ""
}
