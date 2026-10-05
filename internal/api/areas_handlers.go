package api

import (
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The main admin and the other admins' areas (see internal/store/areas.go).
// Each admin route sits behind its area; the settings are split by area too.

var areaNames = map[string]string{
	store.AreaAI: "AI settings", store.AreaDeep: "Deep Scans", store.AreaCalibre: "Calibre & cleanup",
	store.AreaServices: "Email & Discover", store.AreaSystem: "System", store.AreaUsers: "Users",
}

// area admits admins the main admin gave this area (mount after RequireAdmin).
func area(a string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !auth.UserFrom(r).Can(a) {
				writeErr(w, http.StatusForbidden, "the main admin hasn't given you "+areaNames[a])
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// settingArea is the area a setting belongs to: the area of the card it's in
// on the Admin page.
func settingArea(k string) string {
	switch {
	case k == "deep_scan_users" || k == "deep_scan_top_n" || k == "deep_model_ok":
		return store.AreaDeep
	case strings.HasPrefix(k, "calibre_") || k == "format_keep":
		return store.AreaCalibre
	case strings.HasPrefix(k, "smtp_") || k == "nyt_api_key" || strings.HasPrefix(k, "discover_"):
		return store.AreaServices
	case strings.HasPrefix(k, "module_") || strings.HasPrefix(k, "tunnel_") || k == "session_days" ||
		k == "check_updates" || k == "notify_routine":
		return store.AreaSystem
	}
	return store.AreaAI // engines, models, keys, prices, limits, automatic rating, AI machines, language
}

// handleUserAccess: PUT /api/admin/users/{id}/access {"areas": [...]}, the main admin only.
func (s *Server) handleUserAccess(w http.ResponseWriter, r *http.Request) {
	if !auth.UserFrom(r).Owner {
		writeErr(w, http.StatusForbidden, "only the main admin decides what other admins can reach")
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Areas []string `json:"areas"`
	}
	if !readJSON(w, r, &body, 2<<10) {
		return
	}
	target, err := s.Store.UserByID(id)
	if err != nil || target == nil || !target.IsAdmin() || target.Owner {
		writeErr(w, http.StatusBadRequest, "access areas are for admins other than the main admin")
		return
	}
	if err := s.Store.SetAdminAreas(id, body.Areas); err != nil {
		writeStoreErr(w, err)
		return
	}
	u, _ := s.Store.UserByID(id)
	writeJSON(w, http.StatusOK, map[string]any{"areas": u.Areas()})
}

// handleMakeOwner: POST /api/admin/users/{id}/owner, the main admin hands it over.
func (s *Server) handleMakeOwner(w http.ResponseWriter, r *http.Request) {
	if !auth.UserFrom(r).Owner {
		writeErr(w, http.StatusForbidden, "only the main admin can hand it over")
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.Store.MakeOwner(id); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
