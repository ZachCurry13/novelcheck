package api

import (
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

type noteBody struct {
	Body       string `json:"body"`
	Visibility string `json:"visibility"` // everyone | parents
}

// handleAddNote lets an admin or editor leave a note on a book.
func (s *Server) handleAddNote(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body noteBody
	if !readJSON(w, r, &body, 16<<10) {
		return
	}
	if _, err := s.Store.BookByID(id, nil); err != nil {
		writeStoreErr(w, err)
		return
	}
	if _, err := s.Store.AddNote(id, auth.UserFrom(r).ID, body.Body, body.Visibility); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.writeNotes(w, r, id)
}

// ownNote loads note {id} and checks the caller wrote it (admins may manage any).
func (s *Server) ownNote(w http.ResponseWriter, r *http.Request) (*store.BookNote, bool) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return nil, false
	}
	n, err := s.Store.NoteByID(id)
	if err != nil {
		writeStoreErr(w, err)
		return nil, false
	}
	if u := auth.UserFrom(r); n.UserID != u.ID && !u.IsAdmin() {
		writeErr(w, http.StatusForbidden, "you can only change your own notes")
		return nil, false
	}
	return n, true
}

func (s *Server) handleUpdateNote(w http.ResponseWriter, r *http.Request) {
	n, ok := s.ownNote(w, r)
	if !ok {
		return
	}
	var body noteBody
	if !readJSON(w, r, &body, 16<<10) {
		return
	}
	if err := s.Store.UpdateNote(n.ID, body.Body, body.Visibility); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.writeNotes(w, r, n.BookID)
}

func (s *Server) handleDeleteNote(w http.ResponseWriter, r *http.Request) {
	n, ok := s.ownNote(w, r)
	if !ok {
		return
	}
	if err := s.Store.DeleteNote(n.ID); err != nil {
		writeStoreErr(w, err)
		return
	}
	s.writeNotes(w, r, n.BookID)
}

func (s *Server) writeNotes(w http.ResponseWriter, r *http.Request, bookID int64) {
	notes, err := s.Store.BookNotes(bookID, auth.UserFrom(r))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notes": notes})
}

// handleSetAge records a parent's age group for a book (0 clears it).
func (s *Server) handleSetAge(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		AgeLevel int `json:"age_level"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	if !store.ValidAge(body.AgeLevel) {
		writeErr(w, http.StatusBadRequest, "unknown age group")
		return
	}
	if _, err := s.Store.BookByID(id, nil); err != nil {
		writeStoreErr(w, err)
		return
	}
	if err := s.Store.SetBookAge(id, body.AgeLevel, auth.UserFrom(r).Username); err != nil {
		writeStoreErr(w, err)
		return
	}
	b, _ := s.Store.BookByID(id, nil)
	writeJSON(w, http.StatusOK, b)
}

// handleAgeGroups lists the age groups for the UI.
func (s *Server) handleAgeGroups(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, store.AgeGroups)
}
