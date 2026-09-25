package api

// Book covers from the Calibre library (shrunk and cached under /data), and
// reports of wrong covers for an admin to fix in Calibre.

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/covers"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleCover serves a book's cover (?size=l for the book window), or a
// drawn placeholder when Calibre has none.
func (s *Server) handleCover(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	b, err := s.Store.BookByID(id, auth.UserFrom(r))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	width := covers.Small
	if r.URL.Query().Get("size") == "l" {
		width = covers.Large
	}
	// An hour, then the browser checks again: a cover fixed in Calibre shows up soon.
	w.Header().Set("Cache-Control", "private, max-age=3600")
	serve := func(p string, info os.FileInfo) bool {
		f, err := os.Open(p)
		if err != nil {
			return false
		}
		defer f.Close()
		w.Header().Set("Content-Type", "image/jpeg")
		http.ServeContent(w, r, "", info.ModTime(), f)
		return true
	}
	copies, _ := s.Store.BookCopies(id)
	if src, info, found := covers.Find(copies, s.insideCalibre); found {
		if p, err := s.Covers.Thumb(src, info, width); err == nil && serve(p, info) {
			return
		}
	}
	// Not in Calibre (a looked-up or wished-for book): Open Library by ISBN.
	if p, info, found := s.Covers.OpenLibrary(r.Context(), b.ISBN); found && serve(p, info) {
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	_, _ = w.Write(covers.Placeholder(b.Title, b.Author))
}

// handleReportCover lets anyone who can see a book say its cover is wrong.
func (s *Server) handleReportCover(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	u := auth.UserFrom(r)
	if _, err := s.Store.BookByID(id, u); err != nil {
		writeStoreErr(w, err)
		return
	}
	var body struct {
		Note string `json:"note"`
	}
	if r.ContentLength > 0 && !readJSON(w, r, &body, 4<<10) {
		return
	}
	if err := s.Store.ReportCover(id, u, body.Note); err != nil {
		writeStoreErr(w, err)
		return
	}
	s.Store.Notify("info", "covers", "Someone reported a wrong book cover.", "#/admin")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleCoverReports lists the books with open cover reports.
func (s *Server) handleCoverReports(w http.ResponseWriter, r *http.Request) {
	reports, err := s.Store.CoverReports()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reports": reports, "calibre_web_url": s.Store.Setting(store.KeyCalibreWebURL)})
}

// handleCloseCoverReport marks a book's cover reports fixed or dismissed.
func (s *Server) handleCloseCoverReport(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	status := map[string]string{"fixed": "fixed", "dismiss": "dismissed"}[chi.URLParam(r, "action")]
	if !ok || status == "" {
		writeErr(w, http.StatusBadRequest, "use fixed or dismiss")
		return
	}
	closed, err := s.Store.CloseCoverReports(id, status, auth.UserFrom(r).Username)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if !closed {
		writeErr(w, http.StatusNotFound, "that report is already closed")
		return
	}
	if s.Store.CountCoverReports() == 0 {
		s.Store.Resolve("covers")
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
