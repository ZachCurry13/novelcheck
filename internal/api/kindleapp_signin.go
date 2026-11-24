package api

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
	kindleapp "github.com/zachcurry13/novelcheck/kindle-app"
)

// handleAppSignin: POST /kosync/plugin/app-signin. The Kindle app installed from KindleForge has no sign-in file: it asks
// for the NovelCheck name and password once and gets back the key it signs in with from then on (never the sync code itself).
func (s *Server) handleAppSignin(w http.ResponseWriter, r *http.Request) {
	if !s.Store.SettingBool(store.KeyModuleKOReader) {
		kosyncJSON(w, http.StatusForbidden, map[string]string{"message": "Kindle and KOReader delivery is turned off in NovelCheck"})
		return
	}
	if !s.logins.allow(remoteHost(r)) {
		kosyncJSON(w, http.StatusTooManyRequests, map[string]string{"message": "Too many tries. Wait a few minutes."})
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	u, err := s.Store.UserByName(strings.TrimSpace(body.Username))
	if err != nil || u.IsFriend() || !auth.CheckPassword(u.PasswordHash, body.Password) {
		kosyncJSON(w, http.StatusUnauthorized, map[string]string{"message": "That name or password isn't right."})
		return
	}
	code, err := s.Store.KosyncCode(u.ID, false)
	if err != nil {
		kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "Could not sign in."})
		return
	}
	sum := md5.Sum([]byte(code))
	kosyncJSON(w, http.StatusOK, map[string]string{"user": u.Username, "key": hex.EncodeToString(sum[:]), "version": kindleapp.Version})
}
