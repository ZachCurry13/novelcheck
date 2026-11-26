package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Autopilot (off until an admin turns it on from Home): every half hour NovelCheck writes the sure fixes Hardcover
// suggested into Calibre by itself, so nobody has to press Apply. "Sure" is what Fix titles and covers calls sure:
// titles, authors, series, ISBNs and descriptions matched by ISBN or the exact title and author. Genres, series numbers,
// covers, guessed titles and the removal of duplicate copies still wait for a person, in the Needs a decision inbox.
// Calibre has no undo for these writes, which is why it starts off.

const (
	keyAutopilot     = "autopilot"
	keyAutopilotLast = "autopilot_last"
)

// sureSuggestion says whether Autopilot (and the Apply all the sure ones button) may apply it without asking.
func sureSuggestion(x store.HCSuggestion) bool {
	switch x.Field {
	case "title", "authors", "series", "identifiers", "comments":
		return !strings.Contains(x.Confidence, "typo")
	}
	return false
}

func (s *Server) autopilotOn() bool { return s.Store.Setting(keyAutopilot) == "on" }

// sureWaiting counts the Hardcover suggestions Autopilot would apply now.
func (s *Server) sureWaiting() int {
	n := 0
	for _, x := range s.Store.HCSuggestions("new", 5000) {
		if sureSuggestion(x) {
			n++
		}
	}
	return n
}

// AutopilotLoop applies the sure fixes every half hour while Autopilot is on.
func (s *Server) AutopilotLoop(ctx context.Context) {
	tick := time.NewTicker(30 * time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		if s.autopilotOn() {
			s.autopilotOnce()
		}
	}
}

func (s *Server) autopilotOnce() {
	if s.Store.Setting(store.KeyCalibreSrvURL) == "" {
		return
	}
	var sure []store.HCSuggestion
	books := map[int]bool{}
	for _, x := range s.Store.HCSuggestions("new", 5000) {
		if sureSuggestion(x) {
			sure, books[x.CalibreID] = append(sure, x), true
		}
	}
	if len(sure) == 0 || !s.hcjob.start("apply", len(books)) { // nothing to do, or the Hardcover job is busy: next time
		return
	}
	s.runHCApply(sure)
	st := s.hcjob.state()
	msg := fmt.Sprintf("%s: Autopilot wrote %d sure fixes (%d books) into Calibre; %d books could not be done", time.Now().Format("2 Jan 15:04"), len(sure), st.Found, len(st.Errors))
	_ = s.Store.SetSetting(keyAutopilotLast, msg)
	s.Store.NotifyRoutine("autopilot", "Autopilot: "+fmt.Sprintf("%d sure fixes from Hardcover were written into Calibre.", len(sure)), "#/admin?tab=cleanup")
}

// handleAutopilot: GET /api/admin/autopilot, PUT {"on": bool}.
func (s *Server) handleAutopilot(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		var body struct {
			On bool `json:"on"`
		}
		if !readJSON(w, r, &body, 1<<10) {
			return
		}
		v := "off"
		if body.On {
			v = "on"
		}
		if err := s.Store.SetSetting(keyAutopilot, v); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"on": s.autopilotOn(), "waiting": s.sureWaiting(), "last": s.Store.Setting(keyAutopilotLast)})
}
