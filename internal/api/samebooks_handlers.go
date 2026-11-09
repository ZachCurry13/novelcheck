package api

import "net/http"

// Same book, listed twice (Library → Find duplicates): the entries that look
// like one book, to merge or to say are different.

func (s *Server) handleSamePairs(w http.ResponseWriter, r *http.Request) {
	pairs, err := s.Store.SamePairs()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pairs": pairs})
}

// handleSameAction: POST /api/admin/same-books/{merge|not} {"a", "b"}.
func (s *Server) handleSameAction(merge bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			A int64 `json:"a"`
			B int64 `json:"b"`
		}
		if !readJSON(w, r, &body, 1<<10) {
			return
		}
		if body.A <= 0 || body.B <= 0 || body.A == body.B {
			writeErr(w, http.StatusBadRequest, "pick two different books")
			return
		}
		var err error
		if merge {
			_, err = s.Store.MergeBooks(body.A, body.B)
		} else {
			err = s.Store.IgnoreSame(body.A, body.B)
		}
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
