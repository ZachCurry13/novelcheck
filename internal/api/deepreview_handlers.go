package api

import (
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

// Deciding on a held Deep Scan with the book in front of you: the reader
// (the flagged part with the text around it) and a note for parents on the
// book once an admin has decided.

// handleDeepPassage returns a flagged part of a scan's book with the text
// before and after it (?from=&to= in words, or the part's ?label=).
func (s *Server) handleDeepPassage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	book, err := s.Store.DeepReadBook(id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	p, err := s.Deep.Passage(book, queryInt(r, "from"), queryInt(r, "to"), r.URL.Query().Get("label"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// noteDecisions puts a parents' note on each decided scan's book: what the
// scan found and the level the book has now.
func (s *Server) noteDecisions(r *http.Request, ids ...int64) {
	u := auth.UserFrom(r)
	for _, id := range ids {
		book, err := s.Store.DeepReadBook(id)
		if err != nil {
			continue
		}
		if b, err := s.Store.BookByID(book, nil); err == nil && b.SpiceLevel != nil {
			_ = s.Store.NoteDeepDecision(id, u.ID, u.Username, *b.SpiceLevel)
		}
	}
}
