package api

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Family devices ("Who's reading?"). The profile list and the switch are
// open without a session, but only on a device a parent marked; the rest
// needs a signed-in parent.

// pinTries slows guessing: 5 wrong tries lock that profile on that device
// for 5 minutes, and each wrong try after that doubles the wait (up to a day).
type pinTries struct {
	mu    sync.Mutex
	fails map[string]int
	until map[string]time.Time
}

func (p *pinTries) wait(key string) time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.until == nil {
		return 0
	}
	return time.Until(p.until[key])
}

func (p *pinTries) failed(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fails == nil {
		p.fails, p.until = map[string]int{}, map[string]time.Time{}
	}
	p.fails[key]++
	if n := p.fails[key]; n >= 5 {
		lock := 5 * time.Minute << min(n-5, 9)
		p.until[key] = time.Now().Add(min(lock, 24*time.Hour))
	}
}

func (p *pinTries) passed(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.fails, key)
	delete(p.until, key)
}

// needs is what a profile asks for on a family device.
func needs(p store.Profile) string {
	switch {
	case p.HasPin:
		return "pin"
	case p.Role == store.RoleAdmin || p.Role == store.RoleEditor:
		return "password" // a parent without a PIN
	}
	return "none"
}

// handleFamily: GET /api/family, the profiles to pick from (only on a family device).
func (s *Server) handleFamily(w http.ResponseWriter, r *http.Request) {
	d := s.Auth.FamilyDevice(w, r)
	if d == nil {
		writeJSON(w, http.StatusOK, map[string]any{"family": false})
		return
	}
	profiles, err := s.Store.Profiles()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	out := make([]map[string]any, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, map[string]any{"id": p.ID, "username": p.Username, "role": p.Role, "age_level": p.AgeLevel, "needs": needs(p)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"family": true, "name": d.Name, "profiles": out})
}

// handleFamilySwitch: POST /api/family/switch {user_id, pin | password}.
func (s *Server) handleFamilySwitch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserID   int64  `json:"user_id"`
		PIN      string `json:"pin"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	d := s.Auth.FamilyDevice(w, r)
	if d == nil {
		writeErr(w, http.StatusForbidden, "this device isn't set up for the family; sign in with your password")
		return
	}
	u, err := s.Store.UserByID(body.UserID)
	if err != nil || u == nil {
		writeErr(w, http.StatusNotFound, "no such profile")
		return
	}
	key := fmt.Sprintf("%d:%d", d.ID, u.ID)
	if wait := s.pins.wait(key); wait > 0 {
		writeErr(w, http.StatusTooManyRequests, fmt.Sprintf("too many wrong tries; try again in %d minutes", int(wait.Minutes())+1))
		return
	}
	ok := true
	switch needs(store.Profile{Role: u.Role, HasPin: s.Store.PinHash(u.ID) != ""}) {
	case "pin":
		ok = auth.CheckPIN(s.Store.PinHash(u.ID), strings.TrimSpace(body.PIN))
	case "password":
		ok = auth.CheckPassword(u.PasswordHash, body.Password)
	}
	if !ok {
		s.pins.failed(key)
		writeErr(w, http.StatusUnauthorized, "that's not right; try again")
		return
	}
	s.pins.passed(key)
	if err := s.Auth.SwitchTo(w, r, u.ID); err != nil {
		writeStoreErr(w, err)
		return
	}
	out := s.me(u)
	out.FamilyDevice = true
	writeJSON(w, http.StatusOK, out)
}

// handleMakeFamilyDevice: POST (mark this device) or DELETE (unmark it)
// /api/me/family-device, parents only.
func (s *Server) handleMakeFamilyDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		s.Auth.ForgetFamilyDevice(w, r)
		writeJSON(w, http.StatusOK, map[string]bool{"family_device": false})
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || len(name) > 60 {
		name = "Family device"
	}
	if s.Auth.FamilyDevice(nil, r) == nil {
		if err := s.Auth.MakeFamilyDevice(w, r, name, auth.UserFrom(r).Username); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"family_device": true})
}

// handleFamilyDevices: GET /api/admin/family-devices, and DELETE …/{id}.
func (s *Server) handleFamilyDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		id, ok := pathID(r, "id")
		if !ok {
			writeErr(w, http.StatusBadRequest, "invalid id")
			return
		}
		if err := s.Store.RemoveFamilyDevice(id); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	list, err := s.Store.FamilyDevices()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// setPIN sets ("1234") or clears ("") someone's PIN.
func (s *Server) setPIN(w http.ResponseWriter, userID int64, pin string) {
	hash := ""
	if pin = strings.TrimSpace(pin); pin != "" {
		var err error
		if hash, err = auth.HashPIN(pin); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if err := s.Store.SetPinHash(userID, hash); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"has_pin": hash != ""})
}

// handleMyPIN: PUT /api/me/pin {pin, password}; the password proves it's you.
func (s *Server) handleMyPIN(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PIN      string `json:"pin"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	u := auth.UserFrom(r)
	if !auth.CheckPassword(u.PasswordHash, body.Password) {
		writeErr(w, http.StatusForbidden, "your password is incorrect")
		return
	}
	s.setPIN(w, u.ID, body.PIN)
}

// handleUserPIN: PUT /api/admin/users/{id}/pin {pin} (parents; editors only for kids).
func (s *Server) handleUserPIN(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		PIN string `json:"pin"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	target, err := s.Store.UserByID(id)
	if err != nil || target == nil {
		writeErr(w, http.StatusNotFound, "no such user")
		return
	}
	if auth.UserFrom(r).Role != store.RoleAdmin && target.Role != store.RoleRestricted {
		writeErr(w, http.StatusForbidden, "editors can only set kids' PINs")
		return
	}
	s.setPIN(w, target.ID, body.PIN)
}
