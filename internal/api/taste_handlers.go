package api

// The taste profile: an optional list of library books to mark (want to
// read, don't want, read & liked, read & didn't like). The answers are
// saved as suggestion votes and steer Suggested Reads. No AI is used here.

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/suggest"
)

const tasteBatch = 20

// handleTasteBooks returns up to 20 books to mark (?exclude=1,2 skips the
// ones on screen) and the reader's answers so far.
func (s *Server) handleTasteBooks(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	pool, err := s.Store.SuggestPool(u, 0)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	exclude := map[int64]bool{}
	for _, part := range strings.Split(r.URL.Query().Get("exclude"), ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil {
			exclude[id] = true
		}
	}
	books := []*store.Book{}
	for _, b := range suggest.Sample(pool, s.Store.FamilyReads(u.ID), exclude, tasteBatch) {
		if full, err := s.Store.BookByID(b.ID, u); err == nil {
			books = append(books, full)
		}
	}
	marks, err := s.Store.TasteMarks(u.ID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"books": books, "marks": marks})
}

// handleTasteMark saves an answer on a library book (book_id) or, from the
// answers list, a book outside it (title, author). An empty mark removes it.
func (s *Server) handleTasteMark(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BookID int64  `json:"book_id"`
		Title  string `json:"title"`
		Author string `json:"author"`
		Mark   string `json:"mark"` // want, liked, notwant, disliked; "" = remove
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	u := auth.UserFrom(r)
	if body.BookID > 0 {
		b, err := s.Store.BookByID(body.BookID, u)
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		body.Title, body.Author = b.Title, b.Author
	}
	vote := store.ProfileMarks[body.Mark]
	if body.Mark != "" && vote == 0 {
		writeErr(w, http.StatusBadRequest, "unknown answer")
		return
	}
	if err := s.Store.VoteSuggestion(u.ID, body.Title, body.Author, vote, body.Mark); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
