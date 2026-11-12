package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

// handleSetFinished: PUT /api/books/{id}/finished {"day": "2026-03-14"} says you finished the book that day (it
// then counts in Reading stats for that day's year); {"day": ""} takes it out of your finished books. It is
// your own record, beside KOReader's statistics and an imported list.
func (s *Server) handleSetFinished(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Day string `json:"day"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	day := strings.TrimSpace(body.Day)
	if day != "" {
		t, err := time.Parse("2006-01-02", day)
		if err != nil || t.After(time.Now().AddDate(0, 0, 1)) || t.Year() < 1900 {
			writeErr(w, http.StatusBadRequest, "pick the day you finished it (not in the future)")
			return
		}
	}
	if _, err := s.Store.BookByID(id, u); err != nil { // kids' rules apply
		writeStoreErr(w, err)
		return
	}
	if err := s.Store.SetFinishedOn(u.ID, id, day); err != nil {
		writeStoreErr(w, err)
		return
	}
	if day != "" {
		_, _ = s.Store.SyncReading(u.ID, id, true) // ✓ Finished on the Nightstand too
	}
	writeJSON(w, http.StatusOK, map[string]string{"day": s.Store.FinishedOn(u.ID, id)})
}
