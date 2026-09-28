package api

import (
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Paper books: physical libraries (a shelf, a room, a box) that parents fill
// by scanning barcodes one after another, typing a title or ISBN, or taking a
// cover photo. A book NovelCheck doesn't know yet is rated at the front of
// the queue, within the hourly token limit.

// handleCreateShelf makes a physical library ({"name"}); it is the family's.
func (s *Server) handleCreateShelf(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	name, ok := validCatalogName(body.Name)
	if !ok {
		writeErr(w, http.StatusBadRequest, "a library name must be 1-80 characters")
		return
	}
	if store.ReservedCatalogName(name) || strings.EqualFold(name, store.CalibreCatalogName) {
		writeErr(w, http.StatusBadRequest, "that name is taken by NovelCheck; pick another one")
		return
	}
	id, err := s.Store.CreateShelf(name)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "name": name})
}

// handleAddToShelf adds one paper book ({"query"} or {"image"}) to a
// physical library. "added" is false when it was already there.
func (s *Server) handleAddToShelf(w http.ResponseWriter, r *http.Request) {
	shelf, ok := pathID(r, "id")
	if !ok || !s.Store.IsShelf(shelf) {
		writeErr(w, http.StatusNotFound, "no such paper-book library")
		return
	}
	var body struct {
		Query string `json:"query"`
		Image string `json:"image"`
	}
	if !readJSON(w, r, &body, maxPhoto*4/3+4096) {
		return
	}
	look, ok := s.lookUp(w, r, body.Query, body.Image)
	if !ok {
		return
	}
	id := look.id
	if id == 0 {
		f := look.best()
		var err error
		if id, err = s.Store.UpsertBook(f.Title, f.Author, f.ISBN, ""); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	added, err := s.Store.AddToShelf(shelf, id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	b, err := s.Store.BookByID(id, nil)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if b.Status == "pending" || b.Status == "error" {
		s.Worker.Enqueue(true, id)
		b.Status = "queued"
	}
	writeJSON(w, http.StatusOK, map[string]any{"book": b, "added": added, "found": look.found, "source": look.source})
}
