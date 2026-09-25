package api

// "Report a problem or idea": anyone can send one from the app; it goes to
// the admins (🔔 and a list on the Admin page), who can pass a NovelCheck
// bug on through Diagnose. No GitHub account needed.

import (
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/version"
)

func (s *Server) handleReportProblem(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
		Page string `json:"page"`
	}
	if !readJSON(w, r, &body, 16<<10) {
		return
	}
	u := auth.UserFrom(r)
	if err := s.Store.ReportProblem(u, body.Text, body.Page, r.UserAgent(), version.Version); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.Store.Notify("info", "problems", u.Username+" reported a problem or idea.", "#/admin")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleProblemReports(w http.ResponseWriter, r *http.Request) {
	reports, err := s.Store.ProblemReports()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reports": reports})
}

func (s *Server) handleCloseProblemReport(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	closed, err := s.Store.CloseProblemReport(id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if !closed {
		writeErr(w, http.StatusNotFound, "that report is already done")
		return
	}
	if s.Store.CountProblemReports() == 0 {
		s.Store.Resolve("problems")
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
