package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Matching the books on a person's e-readers to the library: the ones that
// found no match (or only a looked-up one) can be matched by hand, or marked
// "not a book" (a thumbnail, KOReader's own guide).

// handleKindleBooks lists the books on the person's e-readers that are not
// matched to the library yet.
func (s *Server) handleKindleBooks(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	s.Store.RematchUnmatched(u.ID)
	s.resolveUnmatched(u.ID)
	items := s.Store.KindleAttention(u.ID)
	type row struct {
		store.Attention
		MatchTitle  string `json:"match_title,omitempty"`
		MatchAuthor string `json:"match_author,omitempty"`
	}
	out := make([]row, 0, len(items))
	for _, a := range items {
		x := row{Attention: a}
		if a.BookID != nil { // looked up in Open Library, not in the library
			if b, err := s.Store.BookByID(*a.BookID, u); err == nil {
				x.MatchTitle, x.MatchAuthor = b.Title, b.Author
			}
		}
		out = append(out, x)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// handleKindleMatch records the person's choice for a book on their e-reader:
// it is this library book ({"book_id": 12}), it is not a book
// ({"ignored": true}), or forget the choice (neither).
func (s *Server) handleKindleMatch(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	var body struct {
		store.MatchRef
		BookID  int64 `json:"book_id"`
		Ignored bool  `json:"ignored"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request")
		return
	}
	if body.Kind != "device" && body.Kind != "koreader" {
		writeErr(w, http.StatusBadRequest, "kind must be device or koreader")
		return
	}
	if body.Kind == "device" && strings.TrimSpace(body.Filename) == "" || body.Kind == "koreader" && strings.TrimSpace(body.Title) == "" {
		writeErr(w, http.StatusBadRequest, "which book?")
		return
	}
	if body.BookID > 0 {
		if _, err := s.Store.BookByID(body.BookID, u); err != nil { // kids' rules still apply
			writeErr(w, http.StatusNotFound, "no such book")
			return
		}
		body.Ignored = false
	}
	if err := s.Store.ApplyMatch(u.ID, body.MatchRef, body.BookID, body.Ignored); err != nil {
		writeStoreErr(w, err)
		return
	}
	added := 0
	if body.BookID > 0 && body.Kind == "device" {
		added = s.autoAddFromDevice(u, body.Device) // the matched book joins Up Next like the others
	}
	writeJSON(w, http.StatusOK, map[string]int{"added": added})
}

// handlePluginNames tells the plugin the proper title and author of the
// device's books that are in the library, so it can tidy the names KOReader shows.
func (s *Server) handlePluginNames(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	names := s.Store.DeviceNames(u.ID, strings.TrimSpace(r.URL.Query().Get("device")))
	kosyncJSON(w, http.StatusOK, map[string]any{"names": names})
}
