package api

import (
	"context"
	"net/http"
	"time"

	"github.com/zachcurry13/novelcheck/internal/deepread"
	"github.com/zachcurry13/novelcheck/internal/safe"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// AI machines (Admin → AI & Scans): model updates, the speed test, and
// whether the Deep Scan machine is answering.

func (s *Server) handleAIToolsStatus(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{"deep_machine": false, "deep_waiting": false, "deep_away": false}
	if s.AITools != nil {
		st := s.AITools.Status()
		out["updates"], out["checked_at"], out["check_error"], out["benching"], out["results"] = st.Updates, st.CheckedAt, st.CheckErr, st.Benching, st.Results
	}
	if _, ok := s.Store.DeepAI(); ok {
		out["deep_machine"] = true
		out["deep_waiting"] = s.Deep != nil && s.Deep.Waiting()
		out["deep_away"] = s.Deep != nil && s.Deep.MachineAway() // asked now (a few seconds at most)
	}
	writeJSON(w, http.StatusOK, out)
}

// handleCheckModelUpdates checks for new model versions in the background.
func (s *Server) handleCheckModelUpdates(w http.ResponseWriter, r *http.Request) {
	if s.AITools == nil {
		writeErr(w, http.StatusServiceUnavailable, "not running")
		return
	}
	safe.Go("model update check", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		s.AITools.CheckUpdates(ctx)
	})
	writeJSON(w, http.StatusAccepted, map[string]bool{"checking": true})
}

// handleModelUpdated forgets an update once the model was downloaded again.
func (s *Server) handleModelUpdated(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Server string `json:"server"`
		Model  string `json:"model"`
	}
	if !readJSON(w, r, &body, 2<<10) {
		return
	}
	if s.AITools != nil {
		s.AITools.Updated(body.Server, body.Model)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleStartBench times one machine's models ({"which": "main" | "backup" | "deep"}).
func (s *Server) handleStartBench(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Which string `json:"which"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	if s.AITools == nil {
		writeErr(w, http.StatusServiceUnavailable, "not running")
		return
	}
	if err := s.AITools.StartBench(body.Which); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"benching": true})
}

// handleRerateBig rates a book the small model wasn't sure of again on the
// Deep Scan machine (in the background; the old rating stays until then).
func (s *Server) handleRerateBig(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	ai, configured := s.Store.DeepAI()
	if !configured {
		writeErr(w, http.StatusBadRequest, "set up a Deep Scan machine first (Admin → AI & Scans → AI machines)")
		return
	}
	if s.Deep != nil && s.Deep.MachineAway() {
		writeErr(w, http.StatusConflict, "the Deep Scan machine isn't answering; switch it on and try again")
		return
	}
	if _, err := s.Store.BookByID(id, nil); err != nil {
		writeStoreErr(w, err)
		return
	}
	safe.Go("re-rating with the Deep Scan machine", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		if err := s.Worker.RateWith(ctx, id, ai); err != nil {
			s.Store.Notify("warning", "rerate-big", "Re-rating on the Deep Scan machine failed: "+err.Error(), "#/library")
		}
	})
	writeJSON(w, http.StatusAccepted, map[string]string{"model": ai.Models[0]})
}

// deepTime estimates how long a Deep Scan of this many words takes, and its
// electricity, from the reading model's speed test (0 when not tested).
func (s *Server) deepTime(e deepread.Estimate) (minutes, power float64) {
	ai, model := s.Deep.Reader()
	b, ok := s.Store.BenchFor(model)
	if !ok || b.PartWords == 0 {
		return 0, 0
	}
	secs := float64(e.Words) / float64(b.PartWords) * b.PartSeconds * 1.15 // second looks at spicy parts
	price := s.Store.SettingFloat(store.KeyPowerPrice)
	return secs / 60, ai.Watts / 1000 * secs / 3600 * price
}

// handleLeaveSafeMode starts the background work that safe mode kept off.
func (s *Server) handleLeaveSafeMode(w http.ResponseWriter, r *http.Request) {
	if err := s.Safe.Leave(); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	s.Store.Resolve("safe-mode")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
