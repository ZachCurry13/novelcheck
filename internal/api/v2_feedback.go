package api

import (
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// "I disagree": a grown-up gives their own level for a book. It is kept next to what the AI said, and the Accuracy page
// (admins) scores the AI against these. It does not change the book's level by itself; an admin decides.

// handleDisagree: POST /api/books/{id}/disagree {"level": 0 to 5, "note"}.
func (s *Server) handleDisagree(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if u.Role == store.RoleRestricted {
		writeErr(w, http.StatusForbidden, "a parent gives the levels")
		return
	}
	if _, err := s.Store.BookByID(id, u); err != nil {
		writeStoreErr(w, err)
		return
	}
	var body struct {
		Level *int   `json:"level"`
		Note  string `json:"note"`
	}
	if !readJSON(w, r, &body, 2<<10) {
		return
	}
	if body.Level == nil || *body.Level < 0 || *body.Level > store.MaxSpiceLevel {
		writeErr(w, http.StatusBadRequest, "pick a level from 0 to 5")
		return
	}
	if err := s.Store.SaveFeedback(u.ID, id, *body.Level, body.Note); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleAccuracy: GET /api/admin/accuracy, how each model did against people's levels, and the newest disagreements.
func (s *Server) handleAccuracy(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"models": s.Store.Accuracy(), "recent": s.Store.RecentFeedback(40)})
}

// handleUseLevel: POST /api/admin/accuracy/{id}/use: the book gets the level a person gave (an admin's decision).
func (s *Server) handleUseLevel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var bookID int64
	var level int
	row := s.Store.DB.QueryRow(`SELECT book_id, level FROM rating_feedback WHERE id = ?`, id)
	if err := row.Scan(&bookID, &level); err != nil {
		writeErr(w, http.StatusNotFound, "that answer is gone")
		return
	}
	if err := s.Store.SetSpiceByHand(bookID, level, auth.UserFrom(r).Username); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
