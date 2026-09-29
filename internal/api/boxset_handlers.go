package api

import (
	"context"
	"net/http"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Box sets: parents check what's inside and split them into their books.

// handleBoxSets finds new box sets and lists those waiting, then split ones.
func (s *Server) handleBoxSets(w http.ResponseWriter, r *http.Request) {
	if _, err := s.Store.FindBoxSets(); err != nil {
		writeStoreErr(w, err)
		return
	}
	list, err := s.Store.BoxSets()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// handleAskBoxSet asks the AI what's inside a box set and keeps the answer
// as the suggestion.
func (s *Server) handleAskBoxSet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	box, err := s.Store.BoxSetFor(id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	book, err := s.Store.BookByID(id, nil)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	var found []store.BoxEntry
	_, err = llm.Ask(ctx, s.Store, s.Store.AIConfigs(), id, llm.BoxContentsSystem,
		llm.BoxContentsUser(box.Title, box.Author, box.Series, book.Description), func(out string) error {
			var perr error
			found, perr = llm.ParseBoxContents(out)
			return perr
		})
	if err != nil {
		writeErr(w, http.StatusBadGateway, "the AI couldn't answer: "+err.Error())
		return
	}
	if err := s.Store.SetBoxProposal(id, found); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"proposal": found})
}

// handleSplitBoxSet splits a box set into the books a parent ticked
// ({"books": [{"title", "number"}]}); new books are rated first in line.
func (s *Server) handleSplitBoxSet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	var body struct {
		Books []store.BoxEntry `json:"books"`
	}
	if !ok || !readJSON(w, r, &body, 64<<10) {
		return
	}
	if len(body.Books) > 30 {
		writeErr(w, http.StatusBadRequest, "at most 30 books")
		return
	}
	unrated, err := s.Store.SplitBox(id, body.Books, auth.UserFrom(r).Username)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(unrated) > 0 {
		s.Worker.Enqueue(true, unrated...)
	}
	writeJSON(w, http.StatusOK, map[string]int{"rating": len(unrated)})
}

// handleBoxSetDecision: "not" (it isn't a box set) or "undo" (put a split
// box set back).
func (s *Server) handleBoxSetDecision(w http.ResponseWriter, r *http.Request, action string) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var err error
	if action == "undo" {
		err = s.Store.UndoSplit(id)
	} else {
		err = s.Store.NotBoxSet(id, auth.UserFrom(r).Username)
	}
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
