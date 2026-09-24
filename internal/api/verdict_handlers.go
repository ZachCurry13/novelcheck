package api

import (
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleSetVerdict lets an admin or editor correct a book's rating by hand
// (e.g. after reading it, or when the AI got it wrong). The model field
// records who made the change so it can be audited later.
func (s *Server) handleSetVerdict(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		SpiceLevel      *int   `json:"spice_level"` // 0-5 peppers (preferred)
		SpiceReason     string `json:"spice_reason"`
		Classification  string `json:"classification"`
		Nudity          bool   `json:"nudity"`
		SoloActs        bool   `json:"solo_acts"`
		HeavyInnuendo   bool   `json:"heavy_innuendo"`
		PlayfulFantasy  bool   `json:"playful_fantasy"`
		DarkOccult      bool   `json:"dark_occult"`
		DemonicPresence bool   `json:"demonic_presence"`
		LGBTQContent    bool   `json:"lgbtq_content"`
		SummaryVerdict  string `json:"summary_verdict"`
	}
	if !readJSON(w, r, &body, 8<<10) {
		return
	}
	if body.SpiceLevel != nil {
		if !store.ValidSpice(*body.SpiceLevel) {
			writeErr(w, http.StatusBadRequest, "peppers must be 0 to 5")
			return
		}
		body.Classification = store.ClassificationForSpice(*body.SpiceLevel)
	}
	if !store.ValidClassification(body.Classification) {
		writeErr(w, http.StatusBadRequest, "choose how many peppers (0-5)")
		return
	}
	if len(body.SummaryVerdict) > 1000 {
		writeErr(w, http.StatusBadRequest, "summary is too long (max 1000 characters)")
		return
	}
	if _, err := s.Store.BookByID(id, nil); err != nil {
		writeStoreErr(w, err)
		return
	}
	a := store.Analysis{
		SpiceLevel:      body.SpiceLevel,
		SpiceReason:     llm.ShortReason(body.SpiceReason),
		Classification:  body.Classification,
		Nudity:          body.Nudity,
		SoloActs:        body.SoloActs,
		HeavyInnuendo:   body.HeavyInnuendo,
		PlayfulFantasy:  body.PlayfulFantasy,
		DarkOccult:      body.DarkOccult || body.DemonicPresence,
		DemonicPresence: body.DemonicPresence,
		LGBTQContent:    body.LGBTQContent,
		SummaryVerdict:  strings.TrimSpace(body.SummaryVerdict),
		Model:           "manual: " + auth.UserFrom(r).Username,
	}
	if err := s.Store.SaveAnalysis(id, a); err != nil {
		writeStoreErr(w, err)
		return
	}
	b, _ := s.Store.BookByID(id, nil)
	writeJSON(w, http.StatusOK, b)
}

// handleSetApproval lets an admin or editor mark a book "OK" so it shows
// even when it matches someone's hide filters or content rules (for
// example Harry Potter's fantasy magic), or remove that mark.
func (s *Server) handleSetApproval(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Approved bool `json:"approved"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	if _, err := s.Store.BookByID(id, nil); err != nil {
		writeStoreErr(w, err)
		return
	}
	if err := s.Store.SetApproved(id, body.Approved, auth.UserFrom(r).Username); err != nil {
		writeStoreErr(w, err)
		return
	}
	b, _ := s.Store.BookByID(id, nil)
	writeJSON(w, http.StatusOK, b)
}
