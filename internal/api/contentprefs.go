package api

import (
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/content"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleContentPrefs: GET /api/me/content-prefs gives the person's Hide / Heads-up choices with every label they can
// choose for; PUT saves them. Kids' accounts keep the rules a parent sets.
func (s *Server) handleContentPrefs(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	if u.Role == store.RoleRestricted {
		writeErr(w, http.StatusForbidden, "a parent sets this account's content rules")
		return
	}
	if r.Method == http.MethodPut {
		var p store.ContentPrefs
		if !readJSON(w, r, &p, 16<<10) {
			return
		}
		if err := s.Store.SaveContentPrefs(u.ID, p); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	flags, _ := s.Store.CustomFlags()
	writeJSON(w, http.StatusOK, map[string]any{"prefs": store.ContentPrefsOf(u), "groups": content.Groups, "flags": flags})
}
