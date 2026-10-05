package api

import (
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

// handleMyOPDS gives the signed-in person their KOReader feed path.
func (s *Server) handleMyOPDS(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	tok, err := s.Store.OPDSToken(u.ID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": "/opds/" + tok, "username": u.Username})
}

// handleResetMyOPDS makes a new feed address (the old one stops working).
func (s *Server) handleResetMyOPDS(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	tok, err := s.Store.ResetOPDSToken(u.ID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": "/opds/" + tok, "username": u.Username})
}

// handleUserOPDS lets a parent set up a kid's e-reader (editors: kid
// accounts only, as everywhere else).
func (s *Server) handleUserOPDS(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	target, err := s.Store.UserByID(id)
	if err != nil || target == nil {
		writeErr(w, http.StatusNotFound, "no such user")
		return
	}
	if !canManageUser(auth.UserFrom(r), target) {
		writeErr(w, http.StatusForbidden, "you can set up kids' devices; the main admin manages the other parents")
		return
	}
	tok, err := s.Store.OPDSToken(target.ID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": "/opds/" + tok, "username": target.Username})
}
