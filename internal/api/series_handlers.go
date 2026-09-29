package api

import (
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
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
