package api

import (
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Parents keep events tidy: rename one or change when it ends, pin it,
// archive or restore it, or delete it.

// handleUpdateEvent renames an event or sets its end ({"name", "ends_at"}:
// RFC 3339, or "" for no end).
func (s *Server) handleUpdateEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	var body struct {
		Name   string `json:"name"`
		EndsAt string `json:"ends_at"`
	}
	if !ok || !readJSON(w, r, &body, 2<<10) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if n := len([]rune(name)); n < 1 || n > 100 {
		writeErr(w, http.StatusBadRequest, "give the event a name (up to 100 characters)")
		return
	}
	ends, good := store.EventTime(body.EndsAt)
	if !good {
		writeErr(w, http.StatusBadRequest, "that end time isn't a date and time")
		return
	}
	if err := s.Store.UpdateEvent(id, name, ends); err != nil {
		writeStoreErr(w, err)
		return
	}
	_, _ = s.Store.ArchiveEnded() // an end already past archives it now
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handlePinEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	var body struct {
		Pinned bool `json:"pinned"`
	}
	if !ok || !readJSON(w, r, &body, 1<<10) {
		return
	}
	if err := s.Store.PinEvent(id, body.Pinned); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"pinned": body.Pinned})
}

// handleArchiveEvent archives an event ({"archived": true}) or restores it.
func (s *Server) handleArchiveEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	var body struct {
		Archived bool `json:"archived"`
	}
	if !ok || !readJSON(w, r, &body, 1<<10) {
		return
	}
	if err := s.Store.ArchiveEvent(id, body.Archived); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"archived": body.Archived})
}

func (s *Server) handleDeleteEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.Store.DeleteEvent(id); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
