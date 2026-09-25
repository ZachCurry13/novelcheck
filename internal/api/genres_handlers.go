package api

// Filling in genres with the AI for library books Calibre has no tags for
// (admins; it spends AI tokens, so it only runs when started here).

import "net/http"

func (s *Server) handleGenreStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Genres.Status())
}

func (s *Server) handleGenreFill(w http.ResponseWriter, r *http.Request) {
	if s.Genres.Status().Missing == 0 {
		writeErr(w, http.StatusBadRequest, "every library book already has a genre")
		return
	}
	if err := s.Genres.Start(); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.Genres.Status())
}

func (s *Server) handleGenreStop(w http.ResponseWriter, r *http.Request) {
	s.Genres.Stop()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
