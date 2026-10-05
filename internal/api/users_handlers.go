package api

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

var usernameRE = regexp.MustCompile(`^[A-Za-z0-9_.-]{2,40}$`)

// Who manages whom: the main admin manages everyone; an admin with the
// Users area manages editors and kids; every other admin and every editor
// manages only kids. Only the main admin makes or removes admins.
func canManageUser(actor, target *store.User) bool {
	switch {
	case actor.Owner:
		return true
	case actor.Can(store.AreaUsers):
		return !target.IsAdmin()
	}
	return actor.CanManage() && target.Role == store.RoleRestricted
}

// managesParents: the main admin, or an admin with the Users area.
func managesParents(u *store.User) bool { return u.Can(store.AreaUsers) }

// targetUser loads the {id} user and checks the caller may manage it.
func (s *Server) targetUser(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return nil, false
	}
	target, err := s.Store.UserByID(id)
	if err != nil {
		writeStoreErr(w, err)
		return nil, false
	}
	if !canManageUser(auth.UserFrom(r), target) {
		writeErr(w, http.StatusForbidden, "you can manage kids' accounts here; the main admin manages the other parents")
		return nil, false
	}
	return target, true
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	var us []store.User
	var err error
	if managesParents(auth.UserFrom(r)) {
		us, err = s.Store.ListUsers()
	} else {
		us, err = s.Store.ListUsersByRole(store.RoleRestricted)
	}
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if us == nil {
		us = []store.User{}
	}
	writeJSON(w, http.StatusOK, us)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		AgeLevel int    `json:"age_level"` // kid accounts: 1 young kids .. 5 adults
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if !usernameRE.MatchString(body.Username) {
		writeErr(w, http.StatusBadRequest, "username must be 2-40 letters, digits, '.', '_' or '-'")
		return
	}
	if !store.ValidRole(body.Role) {
		body.Role = store.RoleRestricted
	}
	actor := auth.UserFrom(r)
	if body.Role == store.RoleAdmin && !actor.Owner {
		writeErr(w, http.StatusForbidden, "only the main admin can make admins")
		return
	}
	if body.Role == store.RoleEditor && !managesParents(actor) {
		writeErr(w, http.StatusForbidden, "you can create kids' accounts; the main admin adds parents")
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !store.ValidAge(body.AgeLevel) {
		writeErr(w, http.StatusBadRequest, "unknown age group")
		return
	}
	u, err := s.Store.CreateUserAge(body.Username, hash, body.Role, body.AgeLevel)
	if err != nil {
		writeErr(w, http.StatusConflict, "username already exists")
		return
	}
	if u.IsAdmin() { // a new admin starts with the default areas
		_ = s.Store.SetAdminAreas(u.ID, []string{store.DefaultAdminAreas})
		u, _ = s.Store.UserByID(u.ID)
	}
	writeJSON(w, http.StatusCreated, u)
}

// handleUpdateUser edits a user's role, content rules and delivery settings.
func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	existing, ok := s.targetUser(w, r)
	if !ok {
		return
	}
	upd := *existing
	if !readJSON(w, r, &upd, 8<<10) {
		return
	}
	upd.ID, upd.Username, upd.PasswordHash = existing.ID, existing.Username, existing.PasswordHash
	if !store.ValidRole(upd.Role) {
		writeErr(w, http.StatusBadRequest, "role must be admin, editor or restricted")
		return
	}
	actor := auth.UserFrom(r)
	if upd.Role != existing.Role && (upd.IsAdmin() || existing.IsAdmin()) && !actor.Owner {
		writeErr(w, http.StatusForbidden, "only the main admin can make or remove admins")
		return
	}
	if upd.Role != store.RoleRestricted && !managesParents(actor) {
		writeErr(w, http.StatusForbidden, "you can manage kids' accounts; the main admin manages parents")
		return
	}
	if existing.Owner && !upd.IsAdmin() {
		writeErr(w, http.StatusBadRequest, "hand over the main admin first (🔑 Make main admin on another admin's card)")
		return
	}
	if existing.IsAdmin() && !upd.IsAdmin() && s.lastAdmin() {
		writeErr(w, http.StatusBadRequest, "cannot demote the last admin")
		return
	}
	if !store.ValidAge(upd.AgeLevel) {
		writeErr(w, http.StatusBadRequest, "unknown age group")
		return
	}
	if upd.MaxSpice < -1 || upd.MaxSpice > store.MaxSpiceLevel {
		writeErr(w, http.StatusBadRequest, "most peppers must be 0 to 5, or no limit")
		return
	}
	if k := invalidRule(upd.HiddenContent); k != "" {
		writeErr(w, http.StatusBadRequest, "unknown content rule "+k)
		return
	}
	if upd.Role != store.RoleRestricted {
		upd.AgeLevel, upd.MaxSpice = 0, -1
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
	if upd.IsAdmin() && !existing.IsAdmin() { // newly an admin: the default areas
		_ = s.Store.SetAdminAreas(upd.ID, []string{store.DefaultAdminAreas})
	}
	if u, err := s.Store.UserByID(upd.ID); err == nil {
		upd.Owner, upd.AdminAreas = u.Owner, u.AdminAreas // never set from the request
	}
	writeJSON(w, http.StatusOK, upd)
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	target, ok := s.targetUser(w, r)
	if !ok {
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
	if err := s.Store.SetPassword(target.ID, hash); err != nil {
		writeStoreErr(w, err)
		return
	}
	_ = s.Store.DeleteUserSessions(target.ID)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	target, ok := s.targetUser(w, r)
	if !ok {
		return
	}
	if target.ID == auth.UserFrom(r).ID {
		writeErr(w, http.StatusBadRequest, "you cannot delete your own account")
		return
	}
	if target.Owner {
		writeErr(w, http.StatusBadRequest, "the main admin can't be deleted; hand it over first")
		return
	}
	if target.IsAdmin() && s.lastAdmin() {
		writeErr(w, http.StatusBadRequest, "cannot delete the last admin")
		return
	}
	if err := s.Store.DeleteUser(target.ID); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) lastAdmin() bool {
	n, err := s.Store.CountAdmins()
	return err != nil || n <= 1
}
