package api

import (
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleRerate re-rates books the AI already rated. They stay visible with
// their old rating until the new one arrives; hand-rated books are left alone.
//
//	which ""         books rated before the pepper scale or under older rules
//	which "language" summaries not in English (when English is chosen)
//	which "all"      every AI-rated book ("Re-rate whole library")
func (s *Server) handleRerate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Which string `json:"which"`
	}
	if r.ContentLength > 0 && !readJSON(w, r, &body, 1<<10) {
		return
	}
	var ids []int64
	var err error
	switch body.Which {
	case "":
		ids, err = s.Store.RerateCandidates()
	case "language":
		ids, err = s.nonEnglishSummaries()
	case "all":
		ids, err = s.Store.AIRatedIDs()
	default:
		writeErr(w, http.StatusBadRequest, `which must be "", "language" or "all"`)
		return
	}
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"queued": s.Worker.Rerate(ids...)})
}

// nonEnglishSummaries finds AI-written summaries that aren't in English, when
// the chosen language is English (hand-rated books are left alone).
func (s *Server) nonEnglishSummaries() ([]int64, error) {
	if !strings.HasPrefix(s.Store.Setting(store.KeyLanguage), "English") {
		return nil, nil
	}
	rows, err := s.Store.AISummaries()
	if err != nil {
		return nil, err
	}
	var ids []int64
	for _, r := range rows {
		if !llm.LooksEnglish(r.Summary) {
			ids = append(ids, r.ID)
		}
	}
	return ids, nil
}

func (s *Server) nonEnglishCount() int {
	ids, _ := s.nonEnglishSummaries()
	return len(ids)
}
