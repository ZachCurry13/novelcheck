package api

import (
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Books that left an e-reader (store/leftbooks.go): the plugin and the Kindle app ask right away; Settings → Kindle &
// KOReader lists the ones still waiting, and the books removed from your library, with Show again.

type leftChoice struct {
	BookID int64  `json:"book_id"`
	Choice string `json:"choice"` // keep | hide | delete
}

func (s *Server) decideLeft(w http.ResponseWriter, u *store.User, c leftChoice, reply func(int, any)) {
	if err := s.Store.DecideLeft(u, c.BookID, c.Choice); err != nil {
		reply(http.StatusBadRequest, map[string]string{"message": err.Error(), "error": err.Error()})
		return
	}
	reply(http.StatusOK, map[string]bool{"ok": true})
}

// handlePluginLeft: POST /kosync/plugin/left {book_id, choice}, from the plugin or the Kindle app.
func (s *Server) handlePluginLeft(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	var c leftChoice
	if !readJSON(w, r, &c, 1<<10) {
		return
	}
	s.decideLeft(w, u, c, func(code int, v any) { kosyncJSON(w, code, v) })
}

// handleMyLeft: GET /api/me/left lists the waiting books and the hidden ones; POST decides one.
func (s *Server) handleMyLeft(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"left": s.Store.LeftBooks(u.ID), "hidden": s.Store.HiddenBooks(u.ID)})
		return
	}
	var c leftChoice
	if !readJSON(w, r, &c, 1<<10) {
		return
	}
	s.decideLeft(w, u, c, func(code int, v any) { writeJSON(w, code, v) })
}

// handleUnhide: DELETE /api/me/hidden-books/{id}, Show again.
func (s *Server) handleUnhide(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.Store.UnhideBook(auth.UserFrom(r).ID, id); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handlePluginHidden: GET /kosync/plugin/hidden, the books this person hid (the Kindle app's "Hidden books").
func (s *Server) handlePluginHidden(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	kosyncJSON(w, http.StatusOK, map[string]any{"books": s.Store.HiddenBooks(u.ID)})
}

// handlePluginUnhide: POST /kosync/plugin/unhide {"book_id"}, Show again.
func (s *Server) handlePluginUnhide(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	var c leftChoice
	if !readJSON(w, r, &c, 1<<10) || c.BookID <= 0 {
		return
	}
	if err := s.Store.UnhideBook(u.ID, c.BookID); err != nil {
		kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "couldn't do that"})
		return
	}
	kosyncJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
