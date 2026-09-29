package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/collections"
	"github.com/zachcurry13/novelcheck/internal/seasons"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Collections: shelves across libraries (manual, AI-filled, or the AI's
// weekly ideas for parents) and the seasonal shelves.

// seasonView is a seasonal shelf with whether it's in season now and the
// collection a parent built for it (0 = its words find the books).
type seasonView struct {
	seasons.Season
	Now        bool  `json:"now"`
	Collection int64 `json:"collection"`
}

func (s *Server) seasonViews() []seasonView {
	now := time.Now()
	out := make([]seasonView, 0, len(seasons.All))
	for _, se := range seasons.All {
		out = append(out, seasonView{Season: se, Now: se.In(now), Collection: s.Store.SeasonCollection(se.Key)})
	}
	return out
}

// seasonFilter narrows a book search to a seasonal shelf: the collection a
// parent built for it, or else its words.
func (s *Server) seasonFilter(f *store.BookFilter, key string) {
	if _, ok := seasons.Find(key); !ok {
		return
	}
	if id := s.Store.SeasonCollection(key); id > 0 && f.Collection == 0 {
		f.Collection = id
		return
	}
	f.Season = key
}

func (s *Server) handleListCollections(w http.ResponseWriter, r *http.Request) {
	cs, err := s.Store.ListCollections(auth.UserFrom(r))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"collections": cs, "seasons": s.seasonViews(),
		"ideas_on": s.Collections != nil && s.Collections.IdeasOn()})
}

func (s *Server) handleGetCollection(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	c, err := s.Store.CollectionByID(id, auth.UserFrom(r))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

type collectionBody struct {
	Name        string           `json:"name"`
	Icon        string           `json:"icon"`
	Description string           `json:"description"`
	Theme       string           `json:"theme"`  // AI collections
	Season      string           `json:"season"` // built for a seasonal shelf
	IDs         []int64          `json:"ids"`    // books to put in it
	Reasons     map[int64]string `json:"reasons"`
}

// valid checks a collection's name and icon.
func (b *collectionBody) valid(w http.ResponseWriter) bool {
	b.Name, b.Icon = strings.TrimSpace(b.Name), strings.TrimSpace(b.Icon)
	if n := len([]rune(b.Name)); n < 1 || n > 80 {
		writeErr(w, http.StatusBadRequest, "a collection's name must be 1-80 characters")
		return false
	}
	if len([]rune(b.Icon)) > 4 {
		b.Icon = "📚"
	}
	if _, ok := seasons.Find(b.Season); !ok {
		b.Season = ""
	}
	if len(b.IDs) > 500 {
		writeErr(w, http.StatusBadRequest, "at most 500 books at a time")
		return false
	}
	return true
}

// handleCreateCollection makes a collection: a parent's own, or (with a
// theme) one the AI filled; ids are its first books.
func (s *Server) handleCreateCollection(w http.ResponseWriter, r *http.Request) {
	var body collectionBody
	if !readJSON(w, r, &body, 64<<10) || !body.valid(w) {
		return
	}
	kind := "manual"
	if strings.TrimSpace(body.Theme) != "" {
		kind = "ai"
	}
	id, err := s.Store.CreateCollection(store.NewCollection{Name: body.Name, Icon: body.Icon, Description: body.Description,
		Kind: kind, Theme: strings.TrimSpace(body.Theme), Season: body.Season, By: auth.UserFrom(r).Username})
	if err == nil {
		_, err = s.Store.AddToCollection(id, body.IDs, body.Reasons)
	}
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (s *Server) handleUpdateCollection(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	var body collectionBody
	if !ok || !readJSON(w, r, &body, 16<<10) || !body.valid(w) {
		return
	}
	if err := s.Store.UpdateCollection(id, body.Name, body.Icon, body.Description); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteCollection(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.Store.DeleteCollection(id); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleKeepIdea keeps one of the AI's ideas as a collection.
func (s *Server) handleKeepIdea(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.Store.KeepIdea(id); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleAddToCollection adds books ({"ids": [...], "reasons": {...}}).
func (s *Server) handleAddToCollection(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	var body collectionBody
	if !ok || !readJSON(w, r, &body, 64<<10) {
		return
	}
	if _, err := s.Store.CollectionByID(id, nil); err != nil {
		writeStoreErr(w, err)
		return
	}
	n, err := s.Store.AddToCollection(id, body.IDs, body.Reasons)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"added": n})
}

func (s *Server) handleRemoveFromCollection(w http.ResponseWriter, r *http.Request) {
	id, ok1 := pathID(r, "id")
	book, ok2 := pathID(r, "book")
	if !ok1 || !ok2 {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.Store.RemoveFromCollection(id, book); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleStartFill has the AI look for books that fit a theme
// ({"theme", "collection_id"}: with a collection, only books not in it yet).
// The app then polls GET /api/collections/ai/{job}.
func (s *Server) handleStartFill(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Theme        string `json:"theme"`
		CollectionID int64  `json:"collection_id"`
		Season       string `json:"season"` // building a seasonal shelf
	}
	if !readJSON(w, r, &body, 8<<10) {
		return
	}
	if s.Collections == nil {
		writeErr(w, http.StatusServiceUnavailable, "AI collections aren't running")
		return
	}
	if strings.TrimSpace(body.Theme) == "" || len(body.Theme) > 500 {
		writeErr(w, http.StatusBadRequest, "describe the collection in a sentence (up to 500 characters)")
		return
	}
	exclude := s.fillExclude(body.CollectionID, body.Season)
	writeJSON(w, http.StatusAccepted, map[string]string{"job": s.Collections.Start(body.Theme, exclude)})
}

func (s *Server) handleFillJob(w http.ResponseWriter, r *http.Request) {
	var job *collections.Job
	if s.Collections != nil {
		job = s.Collections.Job(chi.URLParam(r, "job"))
	}
	if job == nil {
		writeErr(w, http.StatusNotFound, "that search is gone; start it again")
		return
	}
	if job.Picks == nil {
		job.Picks = []collections.Pick{}
	}
	writeJSON(w, http.StatusOK, job)
}
