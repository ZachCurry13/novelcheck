package api

import (
	"log"
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleViewAs: POST /api/admin/users/{id}/view-as. An admin who manages people sees the app as one of the
// family's other accounts (not another admin), read-only, until "Back to my account".
func (s *Server) handleViewAs(w http.ResponseWriter, r *http.Request) {
	admin := auth.UserFrom(r)
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	target, err := s.Store.UserByID(id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	switch {
	case !admin.Can(store.AreaUsers):
		writeErr(w, http.StatusForbidden, "managing people isn't one of your admin areas")
		return
	case target.ID == admin.ID || target.Role == store.RoleAdmin:
		writeErr(w, http.StatusBadRequest, "pick someone who isn't an admin")
		return
	}
	if err := s.Auth.ViewAs(w, r, target.ID, admin.ID); err != nil {
		writeStoreErr(w, err)
		return
	}
	log.Printf("view as: %s is viewing the app as %s", admin.Username, target.Username)
	writeJSON(w, http.StatusOK, map[string]string{"viewing": target.Username})
}

// handleStopViewing: DELETE /api/me/view-as goes back to the admin's own account.
func (s *Server) handleStopViewing(w http.ResponseWriter, r *http.Request) {
	if err := s.Auth.StopViewing(w, r); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// viewedBy names the admin viewing the app as the request's user, or "".
func (s *Server) viewedBy(r *http.Request) string {
	if id := auth.ViewerFrom(r); id != 0 {
		if u, err := s.Store.UserByID(id); err == nil {
			return u.Username
		}
		return "an admin"
	}
	return ""
}
