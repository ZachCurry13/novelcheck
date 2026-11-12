package api

import (
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

// handleReaderPosition: GET /api/books/{id}/position is where you are in the book in NovelCheck's reader, and
// where KOReader's progress sync last put you (so the reader can offer to jump there); PUT {"cfi", "fraction"}
// saves your place.
func (s *Server) handleReaderPosition(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	b, err := s.Store.BookByID(id, u) // kids' rules apply
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if r.Method == http.MethodPut {
		var body struct {
			CFI      string  `json:"cfi"`
			Fraction float64 `json:"fraction"`
		}
		if !readJSON(w, r, &body, 4<<10) {
			return
		}
		if len(body.CFI) > 2000 || body.Fraction < 0 || body.Fraction > 1 || !strings.HasPrefix(body.CFI, "epubcfi(") && body.CFI != "" {
			writeErr(w, http.StatusBadRequest, "not a place in a book")
			return
		}
		if err := s.Store.SaveReaderPosition(u.ID, id, body.CFI, body.Fraction); err != nil {
			writeStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	out := map[string]any{"title": b.Title, "author": b.Author}
	if p, ok := s.Store.ReaderPositionOf(u.ID, id); ok {
		out["here"] = p
	}
	if k, ok := s.Store.KOReaderPositionOf(u.ID, id); ok {
		out["koreader"] = map[string]any{"fraction": k.Percentage, "device": k.Device, "at": k.Timestamp}
	}
	writeJSON(w, http.StatusOK, out)
}
