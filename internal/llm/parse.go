package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Verdict mirrors the JSON contract in the system prompt.
type Verdict struct {
	SpiceRaw        json.RawMessage `json:"spice_level"` // 0-5; models sometimes quote it
	SpiceLevel      *int            `json:"-"`
	SpiceReason     string          `json:"spice_reason"`   // a few words on what sets the level
	Classification  string          `json:"classification"` // older format, used when spice_level is missing
	ContentElements struct {
		Nudity        bool `json:"nudity"`
		SoloActs      bool `json:"solo_acts"`
		HeavyInnuendo bool `json:"heavy_innuendo"`
	} `json:"content_elements"`
	SpiritualElements struct {
		PlayfulFantasy  bool `json:"playful_fantasy"`
		DarkOccult      bool `json:"dark_occult"`
		DemonicPresence bool `json:"demonic_presence"`
	} `json:"spiritual_elements"`
	LGBTQContent   bool            `json:"lgbtq_content"`
	SummaryVerdict string          `json:"summary_verdict"`
	CustomRaw      json.RawMessage `json:"custom_flags"` // the family's own filters: {"key": true} (or a list of keys)
	CustomFlags    []string        `json:"-"`            // keys marked true
}

// ParseVerdict extracts and validates the JSON verdict from model output,
// tolerating markdown code fences and stray prose around the object.
func ParseVerdict(content string) (*Verdict, error) {
	s := strings.TrimSpace(content)
	start, end := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return nil, errors.New("no JSON object in model output")
	}
	var v Verdict
	if err := json.Unmarshal([]byte(s[start:end+1]), &v); err != nil {
		return nil, fmt.Errorf("invalid verdict JSON: %w", err)
	}
	if lvl, ok := spiceFrom(v.SpiceRaw); ok {
		v.SpiceLevel = &lvl
		v.Classification = store.ClassificationForSpice(lvl)
	} else if len(v.SpiceRaw) > 0 && string(v.SpiceRaw) != "null" {
		return nil, fmt.Errorf("spice_level must be 0-5, got %s", v.SpiceRaw)
	}
	v.Classification = normalizeClassification(v.Classification)
	if !store.ValidClassification(v.Classification) {
		return nil, fmt.Errorf("missing spice_level (got classification %q)", v.Classification)
	}
	// Demonic presence implies the dark-occult flag used by parental filters.
	if v.SpiritualElements.DemonicPresence {
		v.SpiritualElements.DarkOccult = true
	}
	v.SummaryVerdict = strings.TrimSpace(v.SummaryVerdict)
	v.SpiceReason = ShortReason(v.SpiceReason)
	v.CustomFlags = customFlagsFrom(v.CustomRaw)
	return &v, nil
}

// maxReasonRunes keeps the pepper reason chip-sized even if a model rambles.
const maxReasonRunes = 80

// ShortReason tidies a pepper reason: one line, no quotes, at most 80 characters.
func ShortReason(s string) string {
	s = strings.Trim(strings.Join(strings.Fields(s), " "), `"'. `)
	if r := []rune(s); len(r) > maxReasonRunes {
		s = strings.TrimSpace(string(r[:maxReasonRunes-1])) + "…"
	}
	return s
}

// spiceFrom reads 3, 3.0, "3" or "3 peppers" as a 0-5 level.
func spiceFrom(raw json.RawMessage) (int, bool) {
	s := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if s == "" || s == "null" {
		return 0, false
	}
	var f float64
	if _, err := fmt.Sscanf(s, "%g", &f); err != nil || f != float64(int(f)) || !store.ValidSpice(int(f)) {
		return 0, false
	}
	return int(f), true
}

func normalizeClassification(c string) string {
	switch strings.ToLower(strings.Join(strings.Fields(strings.NewReplacer("-", " ", "_", " ").Replace(c)), " ")) {
	case "closed door":
		return "Closed Door"
	case "open door":
		return "Open Door"
	case "no spice", "none", "clean":
		return "No Spice"
	}
	return c
}

// ToAnalysis converts a verdict into the persisted form.
func (v *Verdict) ToAnalysis(model string) store.Analysis {
	return store.Analysis{
		SpiceLevel:      v.SpiceLevel,
		SpiceReason:     v.SpiceReason,
		Classification:  v.Classification,
		Nudity:          v.ContentElements.Nudity,
		SoloActs:        v.ContentElements.SoloActs,
		HeavyInnuendo:   v.ContentElements.HeavyInnuendo,
		PlayfulFantasy:  v.SpiritualElements.PlayfulFantasy,
		DarkOccult:      v.SpiritualElements.DarkOccult,
		DemonicPresence: v.SpiritualElements.DemonicPresence,
		LGBTQContent:    v.LGBTQContent,
		SummaryVerdict:  v.SummaryVerdict,
		Model:           model,
		CustomFlags:     v.CustomFlags,
	}
}
