package api

import (
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// "Books on your Kindle NovelCheck doesn't know" (Library cleanup): the book files the Kindle backup found that
// match no book NovelCheck has. Tick the ones to add (they join NovelCheck under their tidy title and author and go on
// the Nightstand) or the ones to leave out. Nothing is added by itself.

// handleKindleUnknown: GET /api/admin/kindle-unknown lists them; POST {"add": [ids], "drop": [ids]} adds or drops.
func (s *Server) handleKindleUnknown(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"books": s.Store.KindleUnknowns(u.ID)})
		return
	}
	var body struct {
		Add  []int64 `json:"add"`
		Drop []int64 `json:"drop"`
	}
	if !readJSON(w, r, &body, 256<<10) {
		return
	}
	added, onShelf := 0, 0
	for _, k := range s.Store.TakeKindleUnknowns(u.ID, body.Add) {
		id, err := s.Store.AddDeviceBook(store.MatchRef{Filename: k.Filename, Title: k.Title, Authors: k.Author})
		if err != nil || id <= 0 {
			continue
		}
		added++
		if s.Store.KindleAutoAdd(u.ID) && s.Store.EnqueueAuto(u.ID, id, "queued") {
			onShelf++
		}
	}
	s.Store.TakeKindleUnknowns(u.ID, body.Drop)
	writeJSON(w, http.StatusOK, map[string]int{"added": added, "on_nightstand": onShelf})
}
