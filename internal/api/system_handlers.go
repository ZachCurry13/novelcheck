package api

import (
	"context"
	"net/http"
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
	if days, err := s.Store.DailyTokens(14); err == nil {
		out["daily_tokens"] = days
	}
	if counts, err := s.Store.StatusCounts(); err == nil {
		out["counts"] = counts
	}
	if u, err := s.Store.Usage(); err == nil {
		out["usage"] = u
		out["cost_spent"] = s.Store.SpentUSD()
	}
	if base := ollamaBase(s.Store.Setting(store.KeyLLMBaseURL)); base != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		info := map[string]any{"url": base}
		if srv, err := ollama.Probe(ctx, base); err != nil {
			info["error"] = "can't reach Ollama at " + base
		} else {
			info["version"] = srv.Version
			info["downloaded"] = srv.Models
			loaded, err := ollama.PS(ctx, base)
			if err != nil {
				info["error"] = err.Error()
			}
			info["loaded"] = loaded
		}
		out["ollama"] = info
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

func (s *Server) handleNotificationsClear(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.ClearNotifications(); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unread": 0, "items": []store.Notification{}})
}
