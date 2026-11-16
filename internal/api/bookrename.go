package api

import (
	"errors"
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleRenameBook: PUT /api/books/{id}/name {"title", "author"}. Parents and
// admins fix the title and author of a book that came from Check a book, a
// list, an event or Discover (a Calibre book is changed in Calibre).
func (s *Server) handleRenameBook(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Title  string `json:"title"`
		Author string `json:"author"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	kept, err := s.Store.RenameBook(id, body.Title, body.Author)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeStoreErr(w, err)
		return
	case err != nil:
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.Store.MarkName(kept, 2) // a parent typed it: the name finder leaves it alone
	writeJSON(w, http.StatusOK, map[string]any{"id": kept, "joined": kept != id})
}
