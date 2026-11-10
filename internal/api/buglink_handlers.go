package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/version"
)

// A secret link to the recent bug reports (the server report with its errors and the debug recording, logs
// sent from e-readers and browsers, AI diagnoses, and the problems and ideas the family reported), so they can
// be read, and deleted once handled, without signing in. An admin
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
	s.Store.NoteBugLinkSeen()
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
	problems, _ := s.Store.ProblemReports()
	fmt.Fprintf(&b, "\n== Problems and ideas reported by the family (open, newest first, %d) ==\n", len(problems))
	for _, p := range problems {
		fmt.Fprintf(&b, "\n--- P%d · %s · version %s · page %s · %s UTC ---\n%s\n(device: %s)\n", p.ID, p.Username, p.Version, orNone(p.Page), p.CreatedAt, p.Text, orNone(p.Device))
	}
	b.WriteString(`
== Handled? ==
Delete what is dealt with: POST <this link's address>/resolve?ids=<numbers> with any of these, joined by &:
  ids=24,25              reports and AI diagnoses by number (all=1: every one)
  errors=E-4d39,E-2de9   those server errors (errors=1: the whole list, slow requests too)
  problems=3,4           family problems and ideas, marked done (problems=all: every open one)
  recording=1            the debug recording
`)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

// handleBugLinkResolve: POST /bugs/{token}/resolve?ids=1,2&errors=1&all=1 deletes handled reports.
func (s *Server) handleBugLinkResolve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !s.Store.BugLinkOK(chi.URLParam(r, "token")) {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	var ids []int64
	for _, p := range strings.Split(q.Get("ids"), ",") {
		if n, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64); err == nil && n > 0 {
			ids = append(ids, n)
		}
	}
	deleted, err := s.Store.DeleteLogs(ids, q.Get("all") == "1")
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	out := map[string]any{"deleted": deleted}
	switch e := strings.TrimSpace(q.Get("errors")); {
	case e == "1":
		out["errors_cleared"] = s.Store.ClearErrors() == nil
	case e != "":
		out["errors_deleted"] = s.Store.DeleteErrors(strings.Split(e, ","))
	}
	if p := strings.TrimSpace(q.Get("problems")); p != "" {
		done := 0
		if p == "all" {
			open, _ := s.Store.ProblemReports()
			for _, pr := range open {
				if ok, _ := s.Store.CloseProblemReport(pr.ID); ok {
					done++
				}
			}
		}
		for _, part := range strings.Split(p, ",") {
			if n, err := strconv.ParseInt(strings.TrimPrefix(strings.TrimSpace(part), "P"), 10, 64); err == nil && n > 0 {
				if ok, _ := s.Store.CloseProblemReport(n); ok {
					done++
				}
			}
		}
		out["problems_done"] = done
	}
	if q.Get("recording") == "1" {
		out["recording_cleared"] = s.Store.ClearActivity() == nil
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) bugLinkJSON(r *http.Request) map[string]any {
	tok := s.Store.BugLinkToken()
	if tok != "" {
		s.Store.BugLinkOK(tok) // dates a link that has none yet
	}
	out := map[string]any{"enabled": tok != "" && !s.Store.BugLinkExpired(), "expired": tok != "" && s.Store.BugLinkExpired()}
	if tok != "" {
		out["url"] = s.baseURL(r) + "/bugs/" + tok
		out["expires"] = s.Store.Setting(store.KeyBugLinkExpires)
		out["seen"] = s.Store.Setting(store.KeyBugLinkSeen)
	}
	return out
}

// handleAdminBugLink: GET shows it, POST turns it on or makes a new one, DELETE turns it off.
func (s *Server) handleAdminBugLink(w http.ResponseWriter, r *http.Request) {
	var err error
	switch r.Method {
	case http.MethodPost:
		if r.URL.Query().Get("extend") == "1" && s.Store.BugLinkToken() != "" {
			err = s.Store.ExtendBugLink()
		} else {
			_, err = s.Store.NewBugLinkToken()
		}
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
