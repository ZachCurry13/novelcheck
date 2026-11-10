package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

// Debug recording: see internal/store/activitylog.go. Everyone's browser asks whether it is on (and shows a
// banner when it is); while it is, browsers send what they did and the server notes every call it answers.

func (s *Server) activityState() map[string]any {
	until := s.Store.ActivityUntil()
	left := 0
	if until > 0 {
		left = int(time.Until(time.Unix(until, 0)).Seconds())
	}
	return map[string]any{"on": until > 0, "until": until, "seconds_left": left}
}

// handleActivityStatus: GET /api/activity/status (any signed-in person).
func (s *Server) handleActivityStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, s.activityState())
}

// handleActivityEvents: POST /api/activity/events {"events":[{"t":ms,"kind","detail"}]} from a browser.
func (s *Server) handleActivityEvents(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Events []struct {
			T      int64  `json:"t"`
			Kind   string `json:"kind"`
			Detail string `json:"detail"`
		} `json:"events"`
	}
	if !readJSON(w, r, &body, 256<<10) {
		return
	}
	if !s.Store.ActivityOn() {
		writeJSON(w, http.StatusOK, s.activityState())
		return
	}
	user := auth.UserFrom(r).Username
	for i, e := range body.Events {
		if i >= 300 {
			break
		}
		kind := e.Kind
		switch kind {
		case "page", "tap", "input", "net", "error", "shown", "shown-error":
		default:
			kind = "note"
		}
		s.Store.AddActivity(time.UnixMilli(e.T), user, "browser", kind, e.Detail)
	}
	writeJSON(w, http.StatusOK, s.activityState())
}

// noteActivity records a call the server answered, when recording is on.
func (s *Server) noteActivity(r *http.Request, status int, took time.Duration) {
	if !s.Store.ActivityOn() {
		return
	}
	p := r.URL.Path
	if strings.HasPrefix(p, "/api/activity/") || strings.HasPrefix(p, "/api/admin/activity") {
		return
	}
	if !(strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/opds") || strings.HasPrefix(p, "/dav") || strings.HasPrefix(p, "/kosync")) {
		return
	}
	q := ""
	if r.URL.RawQuery != "" {
		q = "?" + r.URL.RawQuery
	}
	s.Store.AddActivity(time.Time{}, r.Header.Get("x-auth-user"), "server", "net", fmt.Sprintf("%s %s%s -> %d (%d ms)", r.Method, p, q, status, took.Milliseconds()))
}

// handleActivityStart: PUT /api/admin/activity {"minutes": 5|15|60|0}: on for that long, or off.
func (s *Server) handleActivityStart(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Minutes int `json:"minutes"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	if _, err := s.Store.StartActivity(body.Minutes); err != nil {
		writeStoreErr(w, err)
		return
	}
	who := auth.UserFrom(r).Username
	if body.Minutes > 0 {
		s.Store.AddActivity(time.Time{}, who, "server", "note", fmt.Sprintf("recording started for %d min", min(body.Minutes, 60)))
	} else {
		s.Store.AddActivity(time.Time{}, who, "server", "note", "recording stopped")
	}
	writeJSON(w, http.StatusOK, s.activityState())
}

// activityText is the timeline as plain text, the way it is shown, copied and put in bug reports.
func (s *Server) activityText(n int) string {
	var b strings.Builder
	for _, e := range s.Store.ActivityLog(n) {
		who := e.User
		if who == "" {
			who = "-"
		}
		fmt.Fprintf(&b, "%s UTC %-10s %-7s %-5s %s\n", e.At, who, e.Source, e.Kind, e.Detail)
	}
	return b.String()
}

// handleActivityLog: GET /api/admin/activity (JSON) or ?format=text.
func (s *Server) handleActivityLog(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("format") == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(s.activityText(5000)))
		return
	}
	st := s.activityState()
	st["events"] = s.Store.ActivityLog(500)
	writeJSON(w, http.StatusOK, st)
}

// handleActivityClear: DELETE /api/admin/activity.
func (s *Server) handleActivityClear(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.ClearActivity(); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.activityState())
}

// activitySection puts the latest recorded lines in the server report (and so in bug reports).
func (s *Server) activitySection(line func(string, ...any)) {
	state := "off"
	if s.Store.ActivityOn() {
		state = "ON now"
	}
	line("\n== Debug recording (the last 200 lines; %s) ==", state)
	txt := strings.TrimRight(s.activityText(200), "\n")
	if txt == "" {
		line("(nothing recorded)")
		return
	}
	line("%s", txt)
}
