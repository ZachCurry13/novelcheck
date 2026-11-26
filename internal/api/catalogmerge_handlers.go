package api

import "net/http"

// handleKindleLibraryPairs: GET /api/admin/catalogs/kindle-pairs lists the libraries that look like one person's Kindle twice.
func (s *Server) handleKindleLibraryPairs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"pairs": s.Store.KindleLibraryPairs()})
}

// handleMergeCatalogs: POST /api/admin/catalogs/merge {"from", "into", "name"} joins two libraries of one person.
func (s *Server) handleMergeCatalogs(w http.ResponseWriter, r *http.Request) {
	var body struct {
		From int64  `json:"from"`
		Into int64  `json:"into"`
		Name string `json:"name"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	if err := s.Store.MergeCatalogs(body.From, body.Into, body.Name); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.Store.Notify("info", "libraries", "Two libraries were joined into “"+body.Name+"”.", "#/libraries")
	writeJSON(w, http.StatusOK, map[string]any{"pairs": s.Store.KindleLibraryPairs()})
}
