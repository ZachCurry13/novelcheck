package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/push"
	kindleapp "github.com/zachcurry13/novelcheck/kindle-app"
	kindleplugin "github.com/zachcurry13/novelcheck/kindle-plugin"
)

// Update nudges: when a new Kindle app or KOReader plugin comes out, each person whose Kindle runs an older one gets one phone
// notice (once per version); an admin can send a reminder from the Health list, which also says how far behind each Kindle is.

const nudgeStale = 60 * 24 * time.Hour // a Kindle that has not reported for two months is not nagged

// kindleTarget says what a Kindle's reported version should be: "app" or "plugin", and the newest of that kind.
func kindleTarget(reported string) (kind, current, target string) {
	if v, ok := strings.CutPrefix(reported, "app "); ok {
		return "app", v, kindleapp.Version
	}
	return "plugin", reported, kindleplugin.Version()
}

// versionsBehind counts how many minor versions a is behind b ("1.2.0" behind "1.26.0" is 24); 0 when it is not behind or the
// major versions differ.
func versionsBehind(a, b string) int {
	var a1, a2, b1, b2 int
	fmt.Sscanf(a, "%d.%d", &a1, &a2)
	fmt.Sscanf(b, "%d.%d", &b1, &b2)
	if a1 != b1 || b2 <= a2 {
		return 0
	}
	return b2 - a2
}

func nudgeText(kind, target string) (title, body string) {
	if kind == "app" {
		return "📲 A new NovelCheck Kindle app is ready", "Version " + target + ". Open NovelCheck on your Kindle with Wi-Fi on and it updates itself. If it doesn't, open KindleForge once and try again."
	}
	return "📲 A new NovelCheck KOReader plugin is ready", "Version " + target + ". Open KOReader with Wi-Fi on; NovelCheck updates itself (NovelCheck, Settings, Update if it asks)."
}

func nudgeKey(userID int64, device, kind string) string {
	return fmt.Sprintf("update_nudge_%d_%s_%s", userID, device, kind)
}

// UpdateNudgeLoop looks every hour for Kindles that have not heard about the newest version yet.
func (s *Server) UpdateNudgeLoop(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.nudgeOnce()
		}
	}
}

// nudgeOnce sends the one notice per version to each person with an older Kindle and says how many it sent.
func (s *Server) nudgeOnce() int {
	sent := 0
	for _, d := range s.Store.AllDevices() {
		kind, cur, target := kindleTarget(d.Version)
		if !versionOlder(cur, target) || time.Since(time.Unix(d.Reported, 0)) > nudgeStale {
			continue
		}
		key := nudgeKey(d.UserID, d.Device, kind)
		if s.Store.Setting(key) == target {
			continue // told about this version already
		}
		_ = s.Store.SetSetting(key, target)
		if s.Push != nil {
			title, body := nudgeText(kind, target)
			s.Push.ToUser(d.UserID, push.Message{Title: title, Body: body, URL: "/#/kindle", Tag: "update-" + kind})
		}
		sent++
	}
	return sent
}

// handleRemindDevice: POST /api/admin/devices/remind {"user_id", "device"}: an admin's reminder to that person to update. It says if
// the person has phone notifications on.
func (s *Server) handleRemindDevice(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserID int64  `json:"user_id"`
		Device string `json:"device"`
	}
	if !readJSON(w, r, &body, 2<<10) || body.UserID <= 0 || body.Device == "" {
		writeErr(w, http.StatusBadRequest, "which Kindle?")
		return
	}
	for _, d := range s.Store.AllDevices() {
		if d.UserID != body.UserID || d.Device != body.Device {
			continue
		}
		kind, cur, target := kindleTarget(d.Version)
		if !versionOlder(cur, target) {
			writeErr(w, http.StatusConflict, "that Kindle is up to date")
			return
		}
		subs, _ := s.Store.UserPushSubs(body.UserID)
		if s.Push != nil && len(subs) > 0 {
			title, text := nudgeText(kind, target)
			s.Push.ToUser(body.UserID, push.Message{Title: "Reminder: " + strings.TrimPrefix(title, "📲 "), Body: text, URL: "/#/kindle", Tag: "update-" + kind})
		}
		writeJSON(w, http.StatusOK, map[string]any{"sent": s.Push != nil && len(subs) > 0})
		return
	}
	writeErr(w, http.StatusNotFound, "NovelCheck doesn't know that Kindle")
}
