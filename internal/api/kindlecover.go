package api

import (
	"context"
	"net/http"
	"time"

	"github.com/zachcurry13/novelcheck/internal/covers"
)

// Change cover from the Kindle app, before downloading: the covers found for a book, a picture of each, and the pick.
// It changes the person's own cover (mine-<user>-<book>), like Change cover on the book's web page; an admin
// changes it for everyone there.

// handleKindleCoverOptions: GET /kosync/plugin/cover-options/{id}, up to 6 covers found for the book.
func (s *Server) handleKindleCoverOptions(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	id, _ := pathID(r, "id")
	b, err := s.Store.BookByID(id, u)
	if err != nil {
		kosyncJSON(w, http.StatusNotFound, map[string]string{"message": "that book isn't in your library"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	kosyncJSON(w, http.StatusOK, map[string]any{"candidates": s.Covers.Candidates(ctx, b.Title, b.Author, "", 6)})
}

// handleKindleCoverPreview: GET /kosync/plugin/cover-preview?u=, a candidate's picture (the app needs the sign-in on
// every picture, and the page's own address doesn't allow other sites' pictures).
func (s *Server) handleKindleCoverPreview(w http.ResponseWriter, r *http.Request) {
	if s.kosyncUser(w, r) == nil {
		return
	}
	s.handleCoverPreview(w, r)
}

// handleKindleCover: POST /kosync/plugin/cover {"book_id", "url"} sets your cover; {"book_id", "reset": true} goes
// back to the usual one.
func (s *Server) handleKindleCover(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		BookID int64  `json:"book_id"`
		URL    string `json:"url"`
		Reset  bool   `json:"reset"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	if _, err := s.Store.BookByID(body.BookID, u); err != nil {
		kosyncJSON(w, http.StatusNotFound, map[string]string{"message": "that book isn't in your library"})
		return
	}
	name := myCoverName(u.ID, body.BookID)
	if body.Reset {
		s.Covers.Drop(name)
		kosyncJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if !covers.AllowedImage(body.URL) {
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "not a cover address"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if !s.Covers.Choose(ctx, name, body.URL) {
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "that picture couldn't be used; try another"})
		return
	}
	kosyncJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
