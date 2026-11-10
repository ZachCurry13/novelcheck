package api

import (
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

// handleSetRating: PUT /api/books/{id}/rating {"stars": 4.25} (0 clears). The person's own star rating.
func (s *Server) handleSetRating(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Stars float64 `json:"stars"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	if _, err := s.Store.BookByID(id, u); err != nil { // kids' rules apply
		writeStoreErr(w, err)
		return
	}
	if err := s.Store.SetRating(u.ID, id, body.Stars); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]float64{"stars": s.Store.RatingOf(u.ID, id)})
}
