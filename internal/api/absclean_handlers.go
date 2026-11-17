package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zachcurry13/novelcheck/internal/audiobookshelf"
)

// The Audiobookshelf cleanup's page (js/absclean.js): what the last round did, the switch for fixing by
// itself, "Look now", and the fixes waiting for a parent with Apply and Skip.

var absCleaning sync.Mutex

// handleABSClean: GET /api/admin/audiobookshelf/cleanup.
func (s *Server) handleABSClean(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ABSFixes()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	type row struct {
		ItemID string             `json:"item_id"`
		Title  string             `json:"title"`
		Author string             `json:"author"`
		Why    string             `json:"why"`
		Fix    audiobookshelf.Fix `json:"fix"`
	}
	rows := []row{}
	for _, f := range list {
		x := row{ItemID: f.ItemID, Title: f.Title, Author: f.Author, Why: f.Why}
		_ = json.Unmarshal([]byte(f.Fix), &x.Fix)
		rows = append(rows, x)
	}
	busy := !absCleaning.TryLock()
	if !busy {
		absCleaning.Unlock()
	}
	writeJSON(w, http.StatusOK, map[string]any{"auto": s.Store.Setting(keyABSCleanAuto) != "off", "result": s.Store.Setting(keyABSCleanResult),
		"busy": busy, "fixes": rows, "hardcover": s.Store.RatingsToken() != ""})
}

// handleABSCleanAuto: PUT /api/admin/audiobookshelf/cleanup {"auto": bool}.
func (s *Server) handleABSCleanAuto(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Auto bool `json:"auto"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	v := "on"
	if !body.Auto {
		v = "off"
	}
	if err := s.Store.SetSetting(keyABSCleanAuto, v); err != nil {
		writeStoreErr(w, err)
		return
	}
	s.handleABSClean(w, r)
}

// handleABSCleanRun: POST /api/admin/audiobookshelf/cleanup/run starts a round in the background.
func (s *Server) handleABSCleanRun(w http.ResponseWriter, r *http.Request) {
	go s.cleanAudiobooksNow(500)
	writeJSON(w, http.StatusAccepted, map[string]any{"started": true})
}

func (s *Server) cleanAudiobooksNow(limit int) {
	if !absCleaning.TryLock() {
		return
	}
	defer absCleaning.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	if _, err := s.CleanAudiobooks(ctx, limit); err != nil {
		_ = s.Store.SetSetting(keyABSCleanResult, "Stopped: "+err.Error())
		log.Printf("audiobookshelf cleanup failed: %v", err)
	}
}

// handleABSFixAll: POST /api/admin/audiobookshelf/fixes/all applies every waiting fix (the page asks first). It
// stops at the first refusal for rights, and says how many it applied.
func (s *Server) handleABSFixAll(w http.ResponseWriter, r *http.Request) {
	c, ok := audiobookshelf.FromSettings(s.Store)
	list, err := s.Store.ABSFixes()
	if !ok || err != nil {
		writeErr(w, http.StatusBadRequest, "Audiobookshelf isn't set up")
		return
	}
	applied := 0
	for _, f := range list {
		var fix audiobookshelf.Fix
		if json.Unmarshal([]byte(f.Fix), &fix) != nil {
			continue
		}
		if err := c.ApplyFix(r.Context(), f.ItemID, fix); err != nil {
			if errors.Is(err, audiobookshelf.ErrNoEditRights) {
				writeErr(w, http.StatusForbidden, err.Error())
				return
			}
			log.Printf("audiobookshelf apply all %q: %v", f.Title, err)
			continue
		}
		s.Store.DropABSFix(f.ItemID, false)
		applied++
	}
	if applied > 0 { // NovelCheck's own copy of the details
		res, err := audiobookshelf.Sync(r.Context(), s.Store, c)
		audiobookshelf.Record(s.Store, res, err)
	}
	s.handleABSClean(w, r)
}

// handleABSFix: POST /api/admin/audiobookshelf/fixes/{id} {"action": "apply" | "skip"}.
func (s *Server) handleABSFix(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Action string `json:"action"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	f, ok := s.Store.ABSFixByID(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "that fix is gone")
		return
	}
	if body.Action == "apply" {
		c, ok := audiobookshelf.FromSettings(s.Store)
		var fix audiobookshelf.Fix
		if !ok || json.Unmarshal([]byte(f.Fix), &fix) != nil {
			writeErr(w, http.StatusBadRequest, "Audiobookshelf isn't set up")
			return
		}
		if err := c.ApplyFix(r.Context(), id, fix); err != nil {
			status := http.StatusBadGateway
			if errors.Is(err, audiobookshelf.ErrNoEditRights) {
				status = http.StatusForbidden
			}
			writeErr(w, status, err.Error())
			return
		}
	}
	s.Store.DropABSFix(id, body.Action != "apply")
	s.handleABSClean(w, r)
}
