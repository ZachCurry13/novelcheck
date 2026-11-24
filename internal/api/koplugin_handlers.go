package api

import (
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
	kindleapp "github.com/zachcurry13/novelcheck/kindle-app"
	kindleplugin "github.com/zachcurry13/novelcheck/kindle-plugin"
)

// servePluginZip sends the KOReader plugin already set up for target: its
// server address, username and sync code are in config.lua, so nothing has to
// be typed on the e-reader.
func (s *Server) servePluginZip(w http.ResponseWriter, r *http.Request, target *store.User) {
	if !s.Store.SettingBool(store.KeyModuleKOReader) {
		writeErr(w, http.StatusForbidden, "KOReader is turned off in NovelCheck")
		return
	}
	code, err := s.Store.KosyncCode(target.ID, false)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	data, err := kindleplugin.Zip(s.baseURL(r), target.Username, code)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not build the plugin")
		return
	}
	name := "novelcheck-koplugin-" + strings.ReplaceAll(kindleplugin.Version(), ".", "-") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store") // it holds the sync code
	_, _ = w.Write(data)
}

func (s *Server) handleMyPluginZip(w http.ResponseWriter, r *http.Request) {
	s.servePluginZip(w, r, auth.UserFrom(r))
}

// handleUserPluginZip is the plugin for a kid's device, for their parents.
func (s *Server) handleUserPluginZip(w http.ResponseWriter, r *http.Request) {
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
	s.servePluginZip(w, r, target)
}

func (s *Server) handleMyKindleAppZip(w http.ResponseWriter, r *http.Request) {
	s.serveKindleAppZip(w, r, auth.UserFrom(r))
}

// handleUserKindleAppZip is the Kindle app for someone else's Kindle (a kid, or the family a main admin manages).
func (s *Server) handleUserKindleAppZip(w http.ResponseWriter, r *http.Request) {
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
	s.serveKindleAppZip(w, r, target)
}

// serveKindleAppZip sends the Kindle app (kindle-app/) set up for u: their sign-in is inside, nothing to type.
func (s *Server) serveKindleAppZip(w http.ResponseWriter, r *http.Request, u *store.User) {
	if !s.Store.SettingBool(store.KeyModuleKOReader) {
		writeErr(w, http.StatusForbidden, "Kindle and KOReader delivery is turned off in NovelCheck")
		return
	}
	code, err := s.Store.KosyncCode(u.ID, false)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	data, err := kindleapp.Zip(s.baseURL(r), u.Username, code, r.URL.Query().Get("name"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not build the Kindle app")
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="novelcheck-kindle-app-`+strings.ReplaceAll(kindleapp.Version, ".", "-")+`.zip"`)
	w.Header().Set("Cache-Control", "no-store") // it holds the sign-in
	_, _ = w.Write(data)
}
