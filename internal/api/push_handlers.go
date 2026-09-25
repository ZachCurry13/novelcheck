package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/push"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// pushHosts are the push services browsers use (Chrome/Android, Firefox,
// Safari/iPhone, Edge). Subscriptions may only point at these, so NovelCheck
// never posts to an address inside your network.
var pushHosts = []string{".googleapis.com", ".mozilla.com", ".mozaws.net", ".push.apple.com", ".notify.windows.com"}

func validPushEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, suffix := range pushHosts {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

// handlePushStatus gives the key browsers subscribe with, plus this user's devices.
func (s *Server) handlePushStatus(w http.ResponseWriter, r *http.Request) {
	if s.Push == nil {
		writeErr(w, http.StatusServiceUnavailable, "phone notifications aren't available")
		return
	}
	key, err := s.Push.PublicKey()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "push key: "+err.Error())
		return
	}
	subs, err := s.Store.UserPushSubs(auth.UserFrom(r).ID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	// "key" lets a browser spot its own entry (SHA-256 of its endpoint)
	// without the server handing endpoints back out.
	devices := make([]map[string]any, 0, len(subs))
	for _, d := range subs {
		sum := sha256.Sum256([]byte(d.Endpoint))
		devices = append(devices, map[string]any{"key": hex.EncodeToString(sum[:]), "device": d.Device,
			"scope": d.Scope, "created_at": d.CreatedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"public_key": key, "devices": devices})
}

func (s *Server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
		Scope  string `json:"scope"`
		Device string `json:"device"`
	}
	if !readJSON(w, r, &body, 8<<10) {
		return
	}
	if !validPushEndpoint(body.Endpoint) {
		writeErr(w, http.StatusBadRequest, "this browser's push service isn't supported")
		return
	}
	if !push.ValidKeys(body.Keys.P256dh, body.Keys.Auth) {
		writeErr(w, http.StatusBadRequest, "the browser sent unusable notification keys")
		return
	}
	if body.Scope != "problems" {
		body.Scope = "all"
	}
	if d := []rune(strings.TrimSpace(body.Device)); len(d) > 80 {
		body.Device = string(d[:80])
	}
	sub := store.PushSub{UserID: auth.UserFrom(r).ID, Endpoint: body.Endpoint, P256dh: body.Keys.P256dh,
		Auth: body.Keys.Auth, Scope: body.Scope, Device: strings.TrimSpace(body.Device)}
	if err := s.Store.SavePushSub(sub); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handlePushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Endpoint string `json:"endpoint"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	if err := s.Store.DeletePushSub(auth.UserFrom(r).ID, body.Endpoint); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handlePushTest sends a test notification to the user's own devices now.
func (s *Server) handlePushTest(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	devices, err := s.Store.UserPushSubs(u.ID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if s.Push == nil || len(devices) == 0 {
		writeErr(w, http.StatusBadRequest, "turn on notifications on this device first")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	sent, err := s.Push.SendTo(ctx, devices, push.Message{Title: "NovelCheck", Body: "✓ Notifications work on this device.", URL: "/#/profile", Tag: "test"})
	out := map[string]any{"sent": sent, "devices": len(devices)}
	if err != nil {
		out["error"] = err.Error()
	}
	writeJSON(w, http.StatusOK, out)
}

// pushReading tells the reader their book is on its way (personal notice).
func (s *Server) pushReading(u *store.User, title, note string) {
	if s.Push == nil || strings.Contains(note, "turned off") {
		return
	}
	body := ""
	switch u.DeliveryMethod {
	case "email":
		body = "“" + title + "” was sent to your Kindle. It usually shows up within a few minutes."
	case "koreader":
		body = "“" + title + "” is ready in your KOReader catalog."
	default:
		return
	}
	s.Push.ToUser(u.ID, push.Message{Title: "📚 Ready to read", Body: body, URL: "/#/queue", Tag: "reading"})
}
