package api

// Suggested Reads under Up Next: the suggestions, 👍/👎 votes, and putting a
// suggested book the family doesn't own on the wishlist.

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/safe"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func (s *Server) handleSuggestions(w http.ResponseWriter, r *http.Request) {
	res, err := s.Suggest.For(auth.UserFrom(r))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleSuggestionVote records a 👍 (1) or 👎 (-1), or takes it back (0), on
// a library book (book_id) or a book the family doesn't own (title, author).
func (s *Server) handleSuggestionVote(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BookID int64  `json:"book_id"`
		Title  string `json:"title"`
		Author string `json:"author"`
		Vote   int    `json:"vote"`
		Reason string `json:"reason"` // why a 👎 (optional)
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
	if err := s.Store.VoteSuggestion(u.ID, body.Title, body.Author, body.Vote, body.Reason); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleClearSuggestionVotes forgets all of the reader's 👍/👎.
func (s *Server) handleClearSuggestionVotes(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.ClearSuggestionVotes(auth.UserFrom(r).ID); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleWishSuggestion puts a suggested book the family doesn't own on the
// wishlist. It's looked up like Check a book and rated in the background.
func (s *Server) handleWishSuggestion(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title  string `json:"title"`
		Author string `json:"author"`
		Reason string `json:"reason"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	u := auth.UserFrom(r)
	if u.Role == store.RoleRestricted || u.HideUnrated {
		writeErr(w, http.StatusForbidden, "books from outside the library aren't suggested for this account")
		return
	}
	id := s.Store.MatchBook(body.Title, body.Author)
	if id == 0 {
		f := enrich.Found{Title: body.Title, Author: body.Author}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		if c, ok := s.Worker.Enricher().FindTitle(ctx, body.Title, body.Author); ok && c.Title != "" {
			f = c
		}
		cancel()
		var err error
		if id, err = s.saveLookup(f); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if s.Store.Owned(id) {
		writeErr(w, http.StatusConflict, "this book is already in your library")
		return
	}
	note := "💡 Suggested"
	if body.Reason != "" {
		note += ": " + body.Reason
	}
	if err := s.Store.AddWish(id, u, note); err != nil && !errors.Is(err, store.ErrAlreadyWished) {
		writeStoreErr(w, err)
		return
	}
	_ = s.Store.VoteSuggestion(u.ID, body.Title, body.Author, 1, "")
	if b, err := s.Store.BookByID(id, nil); err == nil {
		s.rateSoon(b)
	}
	writeJSON(w, http.StatusOK, map[string]int64{"book_id": id})
}

// rateSoon rates a looked-up book in the background unless it already is.
func (s *Server) rateSoon(b *store.Book) {
	if b.Status == "analyzed" || b.Status == "processing" {
		return
	}
	id, title := b.ID, b.Title
	safe.Go("rate a looked-up book", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		if err := s.Worker.RateNow(ctx, id); err != nil {
			log.Printf("rating %q failed: %v", title, err)
		}
	})
}
