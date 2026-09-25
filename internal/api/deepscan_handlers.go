package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/deepread"
	"github.com/zachcurry13/novelcheck/internal/ollama"
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
	warning, suggest := s.deepModelWarning(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"scans": scans, "model_warning": warning, "suggest_model": suggest,
		"users": deepread.DeepUsers(s.Store.Setting(store.KeyDeepUsers)), "top_n": s.Store.SettingInt(store.KeyDeepTopN)})
}

// handleDecideDeepScan approves, declines or cancels one scan.
func (s *Server) handleDecideDeepScan(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	by := auth.UserFrom(r).Username
	var done bool
	var err error
	switch action := chi.URLParam(r, "action"); action {
	case "accept": // a held big jump: save its rating
		done, err = s.Store.AcceptDeepRead(id, by)
	case "keep": // a held big jump: keep the rating the book had
		done, err = s.Store.KeepOldRating(id, by)
	default:
		done, err = s.Store.DecideDeepRead(id, action, by)
	}
	if s.Store.HeldDeepReads() == 0 {
		s.Store.Resolve("deep-scan")
	}
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

// deepModelWarning says so when Deep Scan uses a small local model (under
// 7B parameters), which often mistakes tense or violent scenes for romance.
// suggest is the smallest installed model of 7B or more, to switch to in one tap.
func (s *Server) deepModelWarning(ctx context.Context) (warning, suggest string) {
	ais := s.Store.AIConfigs()
	if len(ais) == 0 || len(ais[0].Models) == 0 {
		return "", ""
	}
	model := ais[0].Models[0]
	if m := strings.TrimSpace(s.Store.Setting(store.KeyDeepModel)); m != "" {
		model = m
	}
	base, err := ollama.Normalize(strings.TrimSuffix(strings.TrimSuffix(ais[0].BaseURL, "/"), "/v1"))
	if err != nil {
		return "", ""
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	installed, err := ollama.Installed(ctx, base)
	if err != nil {
		return "", "" // not Ollama (a cloud AI), or not reachable
	}
	size := func(m ollama.Model) float64 {
		b, err := strconv.ParseFloat(strings.TrimSuffix(strings.ToUpper(m.Params), "B"), 64)
		if err != nil {
			return -1
		}
		return b
	}
	var current *ollama.Model
	best := -1.0
	for i, m := range installed {
		if m.Name == model || m.Name == model+":latest" {
			current = &installed[i]
		}
		if b := size(m); b >= 7 && (best < 0 || b < best) {
			best, suggest = b, m.Name
		}
	}
	if current == nil || size(*current) < 0 || size(*current) >= 7 {
		return "", ""
	}
	return fmt.Sprintf("Deep Scan uses %s (%s parameters). Models this small often mistake tense or violent scenes for romance. "+
		"A 7B or bigger model is much more reliable, e.g. qwen2.5:7b or llama3.1:8b (about 5 GB), if your GPU fits it.", model, current.Params), suggest
}
