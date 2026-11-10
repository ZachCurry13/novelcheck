package api

import (
	"net/http"
	"sort"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// GET /api/admin/hc-clean: the background job and the suggestions waiting for review.
func (s *Server) handleHCCleanState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	list := s.Store.HCSuggestions("new", 1500)
	writeJSON(w, http.StatusOK, map[string]any{
		"job": s.hcjob.state(), "suggestions": list, "count": s.Store.CountHCSuggestions(),
		"hardcover": s.Store.HardcoverToken(auth.UserFrom(r).ID) != "", "server": s.Store.Setting(store.KeyCalibreSrvURL) != "",
	})
}

// POST /api/admin/hc-clean/scan starts comparing the Calibre library with the admin's Hardcover.
func (s *Server) handleHCCleanScan(w http.ResponseWriter, r *http.Request) {
	token := s.Store.HardcoverToken(auth.UserFrom(r).ID)
	if token == "" {
		writeErr(w, http.StatusBadRequest, "connect your Hardcover first (Settings → Connections)")
		return
	}
	if s.Store.Setting(store.KeyCalibreSrvURL) == "" {
		writeErr(w, http.StatusBadRequest, "set up the calibre Content server connection first (Admin → Calibre Library)")
		return
	}
	seen := map[int]bool{}
	var refs []store.CalibreBookRef
	for _, ref := range s.Store.CalibreBookRefs() {
		if !seen[ref.CalibreID] {
			seen[ref.CalibreID] = true
			refs = append(refs, ref)
		}
	}
	if len(refs) == 0 {
		writeErr(w, http.StatusBadRequest, "no Calibre books known yet: run a sync first")
		return
	}
	if !s.hcjob.start("scan", len(refs)) {
		writeErr(w, http.StatusConflict, "a Hardcover job is already running")
		return
	}
	go s.runHCScan(token, refs)
	writeJSON(w, http.StatusAccepted, s.hcjob.state())
}

type hcIDs struct {
	IDs []int64 `json:"ids"`
}

// POST /api/admin/hc-clean/apply {"ids": [...]} writes the ticked suggestions to Calibre in the background.
func (s *Server) handleHCCleanApply(w http.ResponseWriter, r *http.Request) {
	var body hcIDs
	if !readJSON(w, r, &body, 64<<10) {
		return
	}
	list := s.Store.HCSuggestionsByID(body.IDs)
	if len(list) == 0 {
		writeErr(w, http.StatusBadRequest, "tick at least one suggestion")
		return
	}
	if s.Store.Setting(store.KeyCalibreSrvURL) == "" {
		writeErr(w, http.StatusBadRequest, "set up the calibre Content server connection first")
		return
	}
	books := map[int]bool{}
	for _, x := range list {
		books[x.CalibreID] = true
	}
	if !s.hcjob.start("apply", len(books)) {
		writeErr(w, http.StatusConflict, "a Hardcover job is already running")
		return
	}
	go s.runHCApply(list)
	writeJSON(w, http.StatusAccepted, s.hcjob.state())
}

// POST /api/admin/hc-clean/skip {"ids": [...]}: these changes are not wanted (they are not suggested again).
func (s *Server) handleHCCleanSkip(w http.ResponseWriter, r *http.Request) {
	var body hcIDs
	if !readJSON(w, r, &body, 64<<10) {
		return
	}
	ids := append([]int64{}, body.IDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	s.Store.SetHCSuggestionStatus(ids, "skipped")
	writeJSON(w, http.StatusOK, map[string]int{"count": s.Store.CountHCSuggestions()})
}

// POST /api/admin/hc-clean/stop asks the running job to stop after the book it is on.
func (s *Server) handleHCCleanStop(w http.ResponseWriter, r *http.Request) {
	s.hcjob.cancel()
	writeJSON(w, http.StatusOK, s.hcjob.state())
}
