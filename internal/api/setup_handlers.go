package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleSetupStatus tells the web page whether to show "Create your admin
// account" (true only while no accounts exist).
func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	n, err := s.Store.CountUsers()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needed": n == 0})
}

// handleSetup creates the first admin from the web page and signs them in.
// It stops working as soon as any account exists.
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if !s.logins.allow(remoteHost(r)) {
		writeErr(w, http.StatusTooManyRequests, "too many attempts; try again in a few minutes")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if !usernameRE.MatchString(body.Username) {
		writeErr(w, http.StatusBadRequest, "username must be 2-40 letters, digits, '.', '_' or '-'")
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.Store.CreateFirstAdmin(body.Username, hash); err != nil {
		if errors.Is(err, store.ErrSetupDone) {
			writeErr(w, http.StatusConflict, "setup is already done; please sign in")
			return
		}
		writeStoreErr(w, err)
		return
	}
	u, err := s.Auth.Login(w, r, body.Username, body.Password)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}
