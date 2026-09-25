package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/deepread"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// deepCost prices an estimate with the main AI's rates.
func (s *Server) deepCost(e deepread.Estimate) float64 {
	ais := s.Store.AIConfigs()
	if len(ais) == 0 {
		return 0
	}
	return float64(e.Tokens-e.Output)*ais[0].PriceIn/1e6 + float64(e.Output)*ais[0].PriceOut/1e6
}

// handleBookDeepScan says whether a book can be Deep Scanned, what it would
// cost, and how its latest scan went (with the notes per part).
func (s *Server) handleBookDeepScan(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.Store.BookByID(id, auth.UserFrom(r)); err != nil {
		writeStoreErr(w, err)
		return
	}
	out := map[string]any{"latest": s.Store.LatestDeepRead(id), "available": false}
	if _, e, err := s.Deep.Prepare(id); err != nil {
		out["why"] = err.Error()
	} else {
		out["available"], out["estimate"], out["cost"] = true, e, s.deepCost(e)
	}
	writeJSON(w, http.StatusOK, out)
}

// handleStartDeepScan: admins start a Deep Scan; anyone else asks for one.
func (s *Server) handleStartDeepScan(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	u := auth.UserFrom(r)
	if _, err := s.Store.BookByID(id, u); err != nil {
		writeStoreErr(w, err)
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if r.ContentLength > 0 && !readJSON(w, r, &body, 4<<10) {
		return
	}
	_, e, err := s.Deep.Prepare(id)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	admin := u.Role == store.RoleAdmin
	source := map[bool]string{true: "admin", false: "request"}[admin]
	if _, err := s.Store.RequestDeepRead(id, source, u.Username, body.Reason, admin, e.Words, e.Parts, e.Tokens); err != nil {
		if errors.Is(err, store.ErrDeepReadOpen) {
			writeErr(w, http.StatusConflict, err.Error())
			return
		}
		writeStoreErr(w, err)
		return
	}
	if admin {
		s.Deep.Wake()
	} else {
		s.Store.Notify("info", "deep-scan-requests", "Books are waiting for your Deep Scan approval.", "#/deepscan")
	}
	writeJSON(w, http.StatusOK, map[string]any{"latest": s.Store.LatestDeepRead(id), "queued": admin})
}

// handleDeepScans is the admin's Deep Scan page: open scans, recent results
// (the audit log of rating changes) and the automatic-scan settings.
func (s *Server) handleDeepScans(w http.ResponseWriter, r *http.Request) {
	scans, err := s.Store.DeepReads(60)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scans": scans,
		"users": deepread.DeepUsers(s.Store.Setting(store.KeyDeepUsers)), "top_n": s.Store.SettingInt(store.KeyDeepTopN)})
}

// handleDecideDeepScan approves, declines or cancels one scan.
func (s *Server) handleDecideDeepScan(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	done, err := s.Store.DecideDeepRead(id, chi.URLParam(r, "action"), auth.UserFrom(r).Username)
	switch {
	case err != nil:
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	case !done:
		writeErr(w, http.StatusConflict, "that Deep Scan has already moved on")
		return
	}
	if s.Store.PendingDeepRequests() == 0 {
		s.Store.Resolve("deep-scan-requests")
	}
	s.Deep.Wake()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleDeepScanNext estimates (GET) or queues (POST) a Deep Scan of the
// next n books waiting in Up Next lists.
func (s *Server) handleDeepScanNext(w http.ResponseWriter, r *http.Request) {
	n := queryInt(r, "n")
	if n <= 0 {
		n = s.Store.SettingInt(store.KeyDeepTopN)
	}
	n = min(max(n, 1), 50)
	ids, e, err := s.Deep.NextInQueues(n)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if r.Method == http.MethodPost {
		queued, qe := s.Deep.Queue(ids, "batch", auth.UserFrom(r).Username)
		writeJSON(w, http.StatusOK, map[string]any{"queued": queued, "estimate": qe, "cost": s.deepCost(qe)})
		return
	}
	titles := make([]string, 0, len(ids))
	for _, id := range ids {
		if b, err := s.Store.BookByID(id, nil); err == nil {
			titles = append(titles, b.Title)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"books": len(ids), "titles": titles, "estimate": e, "cost": s.deepCost(e)})
}

// normalizeDeepUsers keeps up to 3 existing accounts in "deep_scan_users".
func (s *Server) normalizeDeepUsers(v string) string {
	var keep []string
	for _, id := range deepread.DeepUsers(v) {
		if u, err := s.Store.UserByID(id); err == nil && u != nil {
			keep = append(keep, strconv.FormatInt(id, 10))
		}
	}
	return strings.Join(keep, ",")
}
