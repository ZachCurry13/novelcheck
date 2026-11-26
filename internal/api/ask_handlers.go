package api

import (
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/llm"
)

// handleAskBook: POST /api/books/{id}/ask {"question"}: a clarifying question about a rated book ("is the LGBTQ+ character a main
// character or only mentioned?"), answered from the rating, the summary and, when the book was read in full, the notes that
// Deep Scan kept for each part. Kids need the parents' AI-features switch; a book hidden from the person cannot be asked about.
func (s *Server) handleAskBook(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	u := auth.UserFrom(r)
	if !u.MayUseAI() || u.IsFriend() {
		writeErr(w, http.StatusForbidden, "asking the AI about a book is off for your account; ask a parent to turn on AI features")
		return
	}
	b, err := s.Store.BookByID(id, u)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if b.Status != "analyzed" {
		writeErr(w, http.StatusConflict, "this book has not been rated yet; ask again once it has")
		return
	}
	var body struct {
		Question string `json:"question"`
	}
	if !readJSON(w, r, &body, 2<<10) {
		return
	}
	answer, basis, err := s.Deep.Answer(llm.Quick(r.Context()), b, body.Question)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "the AI could not answer: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"answer": answer, "basis": basis})
}
