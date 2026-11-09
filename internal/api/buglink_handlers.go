package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/version"
)

// A secret link to the recent bug reports (the server report, logs sent from
// e-readers and browsers), so they can be read without signing in. An admin
// turns it on, copies it, and turns it off again; the secret is in the
// address, like the OPDS feed's.

const bugLinkLogs, bugLinkMax = 15, 600 << 10

// handleBugLink: GET /bugs/{token}, plain text.
func (s *Server) handleBugLink(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	if !s.Store.BugLinkOK(chi.URLParam(r, "token")) {
		http.NotFound(w, r)
		return
	}
	var b strings.Builder
	b.WriteString(s.diagnosticsText())
	logs, _ := s.Store.RecentLogs(bugLinkLogs)
	fmt.Fprintf(&b, "\n\n== Reports sent from devices and browsers (newest first, %d) ==\n", len(logs))
	for _, l := range logs {
		entry := fmt.Sprintf("\n--- #%d · %s · %s · version %s · %s UTC ---\n%s\n", l.ID, l.Device, l.Username, l.Version, l.CreatedAt, l.Body)
		if b.Len()+len(entry) > bugLinkMax {
			b.WriteString("\n(more reports are in Settings → Developer)\n")
			break
		}
		b.WriteString(entry)
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

func (s *Server) bugLinkJSON(r *http.Request) map[string]any {
	tok := s.Store.BugLinkToken()
	out := map[string]any{"enabled": tok != ""}
	if tok != "" {
		out["url"] = s.baseURL(r) + "/bugs/" + tok
	}
	return out
}

// handleAdminBugLink: GET shows it, POST turns it on or makes a new one, DELETE turns it off.
func (s *Server) handleAdminBugLink(w http.ResponseWriter, r *http.Request) {
	var err error
	switch r.Method {
	case http.MethodPost:
		_, err = s.Store.NewBugLinkToken()
	case http.MethodDelete:
		err = s.Store.ClearBugLink()
	}
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.bugLinkJSON(r))
}

// handleBrowserReport: POST /api/me/bug-report {"text"} keeps a report from this browser.
func (s *Server) handleBrowserReport(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	var body struct {
		Text string `json:"text"`
	}
	if !readJSON(w, r, &body, 256<<10) {
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		writeErr(w, http.StatusBadRequest, "nothing to send")
		return
	}
	device := "Browser"
	if ua := r.UserAgent(); strings.Contains(ua, "Mobile") {
		device = "Browser (phone)"
	}
	if err := s.Store.SaveDeviceLog(u, device, version.Version, time.Now().UTC().Format(time.RFC3339)+"\n"+body.Text); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
