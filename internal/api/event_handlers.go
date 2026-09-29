package api

import (
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/events"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Stuff Your Kindle events: parents paste an event's list (or its page's
// address); everyone sees the books with their ratings, within their rules.

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	_, _ = s.Store.ArchiveEnded() // right away, not only when the loop comes round
	evs, err := s.Store.ListEvents(auth.UserFrom(r))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, evs)
}

func (s *Server) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	_, _ = s.Store.ArchiveEnded()
	u := auth.UserFrom(r)
	ev, err := s.Store.EventByID(id, u)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	books, err := s.Store.EventBooks(id, u)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"event": ev, "books": books})
}

// handlePreviewEvent reads a pasted list ({"html", "text"}) or a page
// ({"url"}, following its "Load more" and next pages) and returns the books
// found, with the event's name and day when the page gives them, without
// saving anything.
func (s *Server) handlePreviewEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		HTML string `json:"html"`
		Text string `json:"text"`
		URL  string `json:"url"`
	}
	if !readJSON(w, r, &body, 8<<20) {
		return
	}
	page := body.HTML
	if strings.TrimSpace(body.URL) != "" && strings.TrimSpace(body.HTML+body.Text) == "" {
		var err error
		if page, err = events.Fetch(r.Context(), body.URL); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	entries := events.Parse(page, body.Text)
	if len(entries) == 0 {
		writeErr(w, http.StatusBadRequest, "no books found; copy the list of books (with its Amazon links, if you can) and paste it here")
		return
	}
	name, day := events.Info(page)
	writeJSON(w, http.StatusOK, map[string]any{"books": entries, "name": name, "day": day})
}

// handleCreateEvent saves an event ({"name", "source_url", "ends_at",
// "books"}) and puts its unrated books first in line to be rated.
func (s *Server) handleCreateEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string `json:"name"`
		SourceURL string `json:"source_url"`
		EndsAt    string `json:"ends_at"`
		Books     []struct {
			Title  string `json:"title"`
			Author string `json:"author"`
			ASIN   string `json:"asin"`
			Link   string `json:"link"`
		} `json:"books"`
	}
	if !readJSON(w, r, &body, 2<<20) {
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
	if len(body.Books) == 0 || len(body.Books) > events.MaxBooks {
		writeErr(w, http.StatusBadRequest, "an event needs 1 to 500 books")
		return
	}
	entries := make([]store.EventEntry, 0, len(body.Books))
	for _, b := range body.Books {
		entries = append(entries, store.EventEntry{Title: b.Title, Author: b.Author, ASIN: b.ASIN, Link: safeLink(b.Link)})
	}
	id, unrated, err := s.Store.CreateEvent(name, safeLink(body.SourceURL), auth.UserFrom(r).Username, ends, entries)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if len(unrated) > 0 {
		s.Worker.Enqueue(true, unrated...) // rated first in line, within the hourly limit
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "rating": len(unrated)})
}

// safeLink keeps only web links (no javascript: and the like).
func safeLink(l string) string {
	l = strings.TrimSpace(l)
	if strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "http://") {
		return l
	}
	return ""
}

// handleClaimEventBook: "✓ I claimed it" puts the book in the chosen library.
func (s *Server) handleClaimEventBook(w http.ResponseWriter, r *http.Request) {
	id, ok1 := pathID(r, "id")
	book, ok2 := pathID(r, "book")
	var body struct {
		CatalogID int64 `json:"catalog_id"`
	}
	if !ok1 || !ok2 || !readJSON(w, r, &body, 1<<10) {
		return
	}
	switch err := s.Store.ClaimEventBook(id, book, body.CatalogID, auth.UserFrom(r)); {
	case err == store.ErrNotLibrary:
		writeErr(w, http.StatusBadRequest, err.Error())
	case err != nil:
		writeStoreErr(w, err)
	default:
		writeJSON(w, http.StatusOK, map[string]bool{"owned": s.Store.Owned(book)})
	}
}
