package api

import (
	"errors"
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/collections"
)

// Keeping shelves on theme (parents): take books off a collection or a
// seasonal shelf for good, and have the AI check a shelf's books.

// maxCheck is the most books one shelf check reads.
const maxCheck = 300

type shelfRef struct {
	CollectionID int64  `json:"collection_id"`
	Season       string `json:"season"`
}

// handleRejectFromShelf: {"collection_id" or "season", "ids"}.
func (s *Server) handleRejectFromShelf(w http.ResponseWriter, r *http.Request) {
	var body struct {
		shelfRef
		IDs []int64 `json:"ids"`
	}
	if !readJSON(w, r, &body, 64<<10) {
		return
	}
	if len(body.IDs) == 0 || len(body.IDs) > 1000 {
		writeErr(w, http.StatusBadRequest, "pick the books to take off the shelf")
		return
	}
	n, err := s.Store.RejectFromShelf(body.CollectionID, body.Season, body.IDs, auth.UserFrom(r).Username)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"removed": n})
}

// handleCheckShelf starts the AI's second look at a shelf's books; the job
// is followed at GET /api/collections/ai/{job}.
func (s *Server) handleCheckShelf(w http.ResponseWriter, r *http.Request) {
	var body shelfRef
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	if s.Collections == nil {
		writeErr(w, http.StatusServiceUnavailable, "AI collections aren't running")
		return
	}
	theme, books, err := s.Store.ShelfToCheck(body.CollectionID, body.Season, maxCheck+1)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if len(books) == 0 {
		writeErr(w, http.StatusBadRequest, "this shelf has no books to check")
		return
	}
	more := len(books) > maxCheck
	books = books[:min(len(books), maxCheck)]
	job, err := s.Collections.StartCheck(theme, books)
	if errors.Is(err, collections.ErrBudget) {
		writeErr(w, http.StatusTooManyRequests, err.Error())
		return
	} else if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job, "total": len(books), "more": more,
		"tokens": collections.CheckTokens(len(books))})
}

// fillExclude is what an AI fill skips: the collection's books, and the
// books a parent took off it (or off the seasonal shelf it's built for).
func (s *Server) fillExclude(collectionID int64, season string) []int64 {
	var out []int64
	if collectionID > 0 {
		out = append(out, s.Store.CollectionBookIDs(collectionID)...)
	}
	if collectionID > 0 || season != "" {
		out = append(out, s.Store.ShelfRejects(collectionID, season)...)
	}
	return out
}
