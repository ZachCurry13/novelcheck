package api

import (
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.logins.allow(remoteHost(r)) {
		writeErr(w, http.StatusTooManyRequests, "too many login attempts; try again in a few minutes")
		return
	}
	body := struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Remember *bool  `json:"remember"` // default true
	}{}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	remember := body.Remember == nil || *body.Remember
	u, err := s.Auth.Login(w, r, strings.TrimSpace(body.Username), body.Password, remember)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	writeJSON(w, http.StatusOK, s.me(u))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.Auth.Logout(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.me(auth.UserFrom(r)))
}

// handleGuideSeen marks the first-login "How to" guide as done (or, with
// {"seen": false}, asks for it to be shown again next time).
func (s *Server) handleGuideSeen(w http.ResponseWriter, r *http.Request) {
	body := struct {
		Seen bool `json:"seen"`
	}{Seen: true}
	if r.ContentLength > 0 && !readJSON(w, r, &body, 1<<10) {
		return
	}
	if err := s.Store.SetGuideSeen(auth.UserFrom(r).ID, body.Seen); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"guide_seen": body.Seen})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	var body struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	if !auth.CheckPassword(u.PasswordHash, body.Current) {
		writeErr(w, http.StatusForbidden, "current password is incorrect")
		return
	}
	hash, err := auth.HashPassword(body.New)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.Store.SetPassword(u.ID, hash); err != nil {
		writeStoreErr(w, err)
		return
	}
	// Invalidate other sessions, then issue a fresh one for this browser.
	_ = s.Store.DeleteUserSessions(u.ID)
	if _, err := s.Auth.Login(w, r, u.Username, body.New, true); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleUpdateDelivery(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	var body struct {
		Method      string `json:"delivery_method"`
		KindleEmail string `json:"kindle_email"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	if msg := validateDelivery(body.Method, body.KindleEmail); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.Store.UpdateDelivery(u.ID, body.Method, strings.TrimSpace(body.KindleEmail)); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func validateDelivery(method, email string) string {
	switch method {
	case "none", "koreader":
	case "email":
		email = strings.TrimSpace(email)
		if !strings.Contains(email, "@") || strings.ContainsAny(email, " \r\n") {
			return "a valid Kindle email address is required for email delivery"
		}
	default:
		return "delivery_method must be none, email or koreader"
	}
	return ""
}
