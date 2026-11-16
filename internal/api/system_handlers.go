package api

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/ollama"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleSystem feeds the Usage page: NovelCheck's resource use and recent
// history, AI token use, library counts, the analysis worker, and
// (when an Ollama server is configured) which models it has loaded.
func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{
		"novelcheck": s.SysInfo.Snapshot(),
		"history":    s.SysInfo.History(),
		"worker":     s.Worker.Status(),
	}
	days := 14 // the token history: 14 days, or up to a year when asked (?days=90)
	if n, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && n >= 7 && n <= 366 {
		days = n
	}
	if days, err := s.Store.DailyTokens(days); err == nil {
		out["daily_tokens"] = days
	}
	if counts, err := s.Store.StatusCounts(); err == nil {
		out["counts"] = counts
	}
	if u, err := s.Store.Usage(); err == nil {
		out["usage"] = u
		out["cost_spent"] = s.Store.SpentUSD()
	}
	// Every Ollama machine set up (your AI, the backup, Deep Scan), each once, asked side by side.
	type machine struct{ name, base string }
	var machines []machine
	seen := map[string]bool{}
	add := func(name, raw string) {
		if base := ollamaBase(raw); base != "" && !seen[base] {
			seen[base] = true
			machines = append(machines, machine{name, base})
		}
	}
	add("Your AI", s.Store.Setting(store.KeyLLMBaseURL))
	for _, ai := range s.Store.AIConfigs() {
		name := map[string]string{"main": "Your AI", "backup": "Backup AI"}[ai.Name]
		if name == "" {
			name = ai.Name + " AI"
		}
		add(name, ai.BaseURL)
	}
	if deep, ok := s.Store.DeepAI(); ok {
		add("Deep Scan", deep.BaseURL)
	}
	infos := make([]map[string]any, len(machines))
	var wg sync.WaitGroup
	for i, m := range machines {
		wg.Add(1)
		go func(i int, m machine) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			info := map[string]any{"url": m.base, "name": m.name}
			if srv, err := ollama.Probe(ctx, m.base); err != nil {
				info["error"] = "can't reach Ollama at " + m.base
			} else {
				info["version"] = srv.Version
				info["downloaded"] = srv.Models
				loaded, err := ollama.PS(ctx, m.base)
				if err != nil {
					info["error"] = err.Error()
				}
				info["loaded"] = loaded
			}
			infos[i] = info
		}(i, m)
	}
	wg.Wait()
	if len(infos) > 0 {
		out["ollama"] = infos[0] // older pages read one
		out["ollamas"] = infos
	}
	writeJSON(w, http.StatusOK, out)
}

// handleHealthChecks runs every connection check (AI provider, book APIs,
// Calibre, email, remote access, Ollama, updates).
func (s *Server) handleHealthChecks(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute) // local AI checks may wait for a model to load
	defer cancel()
	writeJSON(w, http.StatusOK, map[string]any{
		"checked_at": time.Now().UTC().Format(time.RFC3339),
		"results":    s.runChecks(ctx),
	})
}

func (s *Server) handleNotifications(w http.ResponseWriter, r *http.Request) {
	items, unread, err := s.Store.Notifications(100)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if items == nil {
		items = []store.Notification{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"unread": unread, "items": items})
}

// handleNotificationsRead marks one ({"id": n}) or all ({}) as read.
func (s *Server) handleNotificationsRead(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID int64 `json:"id"`
	}
	if r.ContentLength > 0 && !readJSON(w, r, &body, 1<<10) {
		return
	}
	if err := s.Store.MarkNotificationsRead(body.ID); err != nil {
		writeStoreErr(w, err)
		return
	}
	s.handleNotifications(w, r)
}

// handleNotificationsDismiss deletes notices ({"ids": [1, 2]}): the ✕ on one,
// or a whole group such as the Deep Scans waiting for review.
func (s *Server) handleNotificationsDismiss(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if !readJSON(w, r, &body, 16<<10) {
		return
	}
	if err := s.Store.DismissNotifications(body.IDs); err != nil {
		writeStoreErr(w, err)
		return
	}
	s.handleNotifications(w, r)
}

func (s *Server) handleNotificationsClear(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.ClearNotifications(); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unread": 0, "items": []store.Notification{}})
}
