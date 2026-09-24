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
	Classification  string `json:"classification"`
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
	LGBTQContent   bool   `json:"lgbtq_content"`
	SummaryVerdict string `json:"summary_verdict"`
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
	v.Classification = normalizeClassification(v.Classification)
	if !store.ValidClassification(v.Classification) {
		return nil, fmt.Errorf("unknown classification %q", v.Classification)
	}
	// Demonic presence implies the dark-occult flag used by parental filters.
	if v.SpiritualElements.DemonicPresence {
		v.SpiritualElements.DarkOccult = true
	}
	v.SummaryVerdict = strings.TrimSpace(v.SummaryVerdict)
	return &v, nil
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
	}
}
