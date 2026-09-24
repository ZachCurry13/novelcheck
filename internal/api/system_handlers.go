package api

import (
	"context"
	"net/http"
	"time"

	"github.com/zachcurry13/novelcheck/internal/ollama"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleSystem reports NovelCheck's resource use, the analysis worker, and
// (when an Ollama server is configured) which models it has loaded.
func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{
		"novelcheck": s.SysInfo.Snapshot(),
		"worker":     s.Worker.Status(),
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
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
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
