package api

import (
	"net/http"

	"github.com/zachcurry13/novelcheck"
	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleUpdates reports the running version, whether a newer release exists
// (admins and editors only), GitHub release notes, and the bundled changelog.
func (s *Server) handleUpdates(w http.ResponseWriter, r *http.Request) {
	enabled := s.Store.SettingBool(store.KeyCheckUpdates) && s.Updates != nil
	var st any
	if s.Updates != nil {
		status := s.Updates.Status(r.Context(), enabled)
		if !auth.UserFrom(r).CanManage() {
			status.UpdateAvailable = false // kids don't need update nags
		}
		st = status
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         st,
		"checks_enabled": enabled,
		"changelog":      novelcheck.Changelog,
	})
}
