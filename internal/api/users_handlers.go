package api

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

var usernameRE = regexp.MustCompile(`^[A-Za-z0-9_.-]{2,40}$`)

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	us, err := s.Store.ListUsers()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, us)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if !usernameRE.MatchString(body.Username) {
		writeErr(w, http.StatusBadRequest, "username must be 2-40 letters, digits, '.', '_' or '-'")
		return
	}
	if body.Role != store.RoleAdmin {
		body.Role = store.RoleRestricted
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	u, err := s.Store.CreateUser(body.Username, hash, body.Role)
	if err != nil {
		writeErr(w, http.StatusConflict, "username already exists")
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

// handleUpdateUser edits a user's role, content rules and delivery settings.
func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := s.Store.UserByID(id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	upd := *existing
	if !readJSON(w, r, &upd, 8<<10) {
		return
	}
	upd.ID, upd.Username, upd.PasswordHash = existing.ID, existing.Username, existing.PasswordHash
	if upd.Role != store.RoleAdmin && upd.Role != store.RoleRestricted {
		writeErr(w, http.StatusBadRequest, "role must be admin or restricted")
		return
	}
	if existing.IsAdmin() && !upd.IsAdmin() && s.lastAdmin() {
		writeErr(w, http.StatusBadRequest, "cannot demote the last admin")
		return
	}
	if msg := validateDelivery(upd.DeliveryMethod, upd.KindleEmail); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	upd.KindleEmail = strings.TrimSpace(upd.KindleEmail)
	if err := s.Store.UpdateUserProfile(&upd); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, upd)
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.Store.UserByID(id); err != nil {
		writeStoreErr(w, err)
		return
	}
	if err := s.Store.SetPassword(id, hash); err != nil {
		writeStoreErr(w, err)
		return
	}
	_ = s.Store.DeleteUserSessions(id)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if id == auth.UserFrom(r).ID {
		writeErr(w, http.StatusBadRequest, "you cannot delete your own account")
		return
	}
	target, err := s.Store.UserByID(id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if target.IsAdmin() && s.lastAdmin() {
		writeErr(w, http.StatusBadRequest, "cannot delete the last admin")
		return
	}
	if err := s.Store.DeleteUser(id); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) lastAdmin() bool {
	n, err := s.Store.CountAdmins()
	return err != nil || n <= 1
}
