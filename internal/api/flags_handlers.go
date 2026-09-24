package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleListFlags lists the family's custom AI filters (for chips and the
// Library's Hide checkboxes).
func (s *Server) handleListFlags(w http.ResponseWriter, r *http.Request) {
	flags, err := s.Store.CustomFlags()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, flags)
}

type flagBody struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// read validates a filter's name (1-40 characters) and instructions (up to 300).
func (b *flagBody) read(w http.ResponseWriter, r *http.Request) bool {
	if !readJSON(w, r, b, 4<<10) {
		return false
	}
	b.Label = strings.Join(strings.Fields(b.Label), " ")
	b.Description = strings.TrimSpace(b.Description)
	switch {
	case b.Label == "" || len([]rune(b.Label)) > 40:
		writeErr(w, http.StatusBadRequest, "give the filter a name of up to 40 characters")
	case len([]rune(b.Description)) > 300:
		writeErr(w, http.StatusBadRequest, "keep the instructions under 300 characters")
	default:
		return true
	}
	return false
}

func (s *Server) handleAddFlag(w http.ResponseWriter, r *http.Request) {
	var b flagBody
	if !b.read(w, r) {
		return
	}
	f, err := s.Store.AddCustomFlag(b.Label, b.Description)
	if errors.Is(err, store.ErrTooManyFlags) {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

func (s *Server) handleUpdateFlag(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var b flagBody
	if !b.read(w, r) {
		return
	}
	if err := s.Store.UpdateCustomFlag(id, b.Label, b.Description); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteFlag(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.Store.DeleteCustomFlag(id); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
