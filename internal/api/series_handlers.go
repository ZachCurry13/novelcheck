package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/hardcover"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Series: every series the family has, and one series in order with the
// numbers missing and what to read next.

func (s *Server) handleSeriesList(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.SeriesList(auth.UserFrom(r))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// handleSeries returns one series (?name=): its books in order, the gaps and
// the book to read next.
func (s *Server) handleSeries(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeErr(w, http.StatusBadRequest, "which series?")
		return
	}
	books, err := s.Store.SeriesBooks(name, auth.UserFrom(r))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if len(books) == 0 {
		writeErr(w, http.StatusNotFound, "no books of that series here")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": books[0].Series, "books": books,
		"gaps": store.SeriesGaps(books), "next": store.NextInSeries(books)})
}

// handleSameAs makes this book and another one ({"other_id"}) one book, and
// returns the one kept.
func (s *Server) handleSameAs(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	var body struct {
		OtherID int64 `json:"other_id"`
	}
	if !ok || !readJSON(w, r, &body, 1<<10) {
		return
	}
	kept, err := s.Store.MergeBooks(id, body.OtherID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"kept": kept})
}

// handleSeriesFill: POST /api/series/fill?name= looks the series up at Hardcover (with this person's token, or
// a family member's when they have none) and adds the books the family does not have as looked-up books in
// their place in the series, so each can be requested or found at a Libby library.
func (s *Server) handleSeriesFill(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	u := auth.UserFrom(r)
	if name == "" {
		writeErr(w, http.StatusBadRequest, "which series?")
		return
	}
	books, err := s.Store.SeriesBooks(name, u)
	if err != nil || len(books) == 0 {
		writeErr(w, http.StatusNotFound, "no books of that series here")
		return
	}
	token := s.Store.HardcoverToken(u.ID)
	if token == "" {
		for _, id := range s.Store.HardcoverUsers() { // anyone's token will do for a public series list
			if token = s.Store.HardcoverToken(id); token != "" {
				break
			}
		}
	}
	if token == "" {
		writeErr(w, http.StatusBadRequest, "add a Hardcover token in Settings → Connections first: the list of a series' books comes from Hardcover")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	entries, err := (&hardcover.Client{Token: token, URL: hardcoverURL}).SeriesBooks(ctx, books[0].Series)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	fills := make([]store.SeriesFill, 0, len(entries))
	for _, e := range entries {
		fills = append(fills, store.SeriesFill{Position: e.Position, Title: e.Title, Author: e.Author, CoverURL: e.CoverURL})
	}
	added, err := s.Store.AddSeriesBooks(books[0].Series, s.Store.SeriesAuthor(books[0].Series), fills)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	s.rcache.bump()
	writeJSON(w, http.StatusOK, map[string]int{"found": len(entries), "added": added})
}
