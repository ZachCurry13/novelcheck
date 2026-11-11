package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/version"
	kindleplugin "github.com/zachcurry13/novelcheck/kindle-plugin"
)

// Admin → Integrations: the requests behind the page (the rows are built in integrations.go).

// pluginDevices says which plugin version each e-reader last reported.
func (s *Server) pluginDevices() string {
	var rows []struct {
		Device string `db:"device"`
		Plugin string `db:"plugin_version"`
	}
	_ = s.Store.DB.Select(&rows, `SELECT device, plugin_version FROM device_info WHERE plugin_version != '' ORDER BY reported_at DESC LIMIT 6`)
	if len(rows) == 0 {
		return "No e-reader has reported its plugin version yet."
	}
	parts := make([]string, 0, len(rows))
	behind := false
	for _, r := range rows {
		parts = append(parts, r.Device+": "+r.Plugin)
		behind = behind || r.Plugin != kindleplugin.Version()
	}
	out := "On your e-readers: " + strings.Join(parts, " · ") + "."
	if behind {
		out += " An older one updates from its own menu: NovelCheck → Update plugin."
	}
	return out
}

func (s *Server) integrationsJSON(ctx context.Context) map[string]any {
	s.integ.mu.Lock()
	at, running := s.integ.checkedAt, s.integ.running
	s.integ.mu.Unlock()
	out := map[string]any{"groups": s.integrations(ctx), "checking": running, "version": version.Version}
	if !at.IsZero() {
		out["checked_at"] = at.UTC().Format(time.RFC3339)
	}
	return out
}

// handleIntegrations: GET /api/admin/integrations.
func (s *Server) handleIntegrations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, s.integrationsJSON(r.Context()))
}

// handleIntegrationsCheck: POST /api/admin/integrations/check starts the connection checks in the background
// (a local model may take a minute to answer); the page asks again until "checking" is false.
func (s *Server) handleIntegrationsCheck(w http.ResponseWriter, r *http.Request) {
	s.integ.mu.Lock()
	if !s.integ.running {
		s.integ.running = true
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			res := s.runChecks(ctx)
			s.integ.mu.Lock()
			s.integ.checks, s.integ.checkedAt, s.integ.running = res, time.Now(), false
			s.integ.mu.Unlock()
		}()
	}
	s.integ.mu.Unlock()
	writeJSON(w, http.StatusAccepted, s.integrationsJSON(r.Context()))
}

// handleDatabaseCheck: POST /api/admin/integrations/db-check runs SQLite's own check of the file.
func (s *Server) handleDatabaseCheck(w http.ResponseWriter, r *http.Request) {
	var lines []string
	if err := s.Store.DB.SelectContext(r.Context(), &lines, `PRAGMA quick_check`); err != nil {
		writeStoreErr(w, err)
		return
	}
	ok := len(lines) == 1 && lines[0] == "ok"
	msg := "The database file is sound."
	if !ok {
		msg = "SQLite reports: " + strings.Join(lines[:min(len(lines), 5)], "; ") + ". Download a backup now (System & Toggles) and report this."
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": ok, "message": msg})
}
