package api

import (
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleBookmarks: GET lists your bookmarks in a book, POST {cfi, fraction, label} adds one, DELETE {cfi}
// takes one away. The book must be one you may open (kids' rules apply).
func (s *Server) handleBookmarks(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.Store.BookByID(id, u); err != nil {
		writeStoreErr(w, err)
		return
	}
	if r.Method != http.MethodGet {
		var b store.Bookmark
		if !readJSON(w, r, &b, 4<<10) {
			return
		}
		if !strings.HasPrefix(b.CFI, "epubcfi(") || len(b.CFI) > 2000 || len(b.Label) > 200 || b.Fraction < 0 || b.Fraction > 1 {
			writeErr(w, http.StatusBadRequest, "not a place in a book")
			return
		}
		var err error
		if r.Method == http.MethodDelete {
			err = s.Store.RemoveBookmark(u.ID, id, b.CFI)
		} else {
			err = s.Store.AddBookmark(u.ID, id, b)
		}
		if err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	list, err := s.Store.Bookmarks(u.ID, id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"bookmarks": list})
}
