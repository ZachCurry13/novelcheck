package api

// Filling in genres with the AI for library books Calibre has no tags for
// (admins; it spends AI tokens, so it only runs when started here).

import (
	"context"
	"net/http"
)

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

// handleGenreHardcover: POST /api/admin/genres/hardcover looks the books with no genre up on Hardcover now (in the
// background), instead of waiting for the next round.
func (s *Server) handleGenreHardcover(w http.ResponseWriter, r *http.Request) {
	if s.Store.RatingsToken() == "" {
		writeErr(w, http.StatusBadRequest, "no one has connected Hardcover yet (Settings → Connections)")
		return
	}
	if !s.ratingBusy.CompareAndSwap(false, true) {
		writeErr(w, http.StatusConflict, "Hardcover is already being looked at; give it a few minutes")
		return
	}
	s.Store.RecheckGenreless()
	s.Store.RecheckAIGenres()
	go func() {
		defer s.ratingBusy.Store(false)
		s.FillRatings(context.Background(), 800)
	}()
	writeJSON(w, http.StatusAccepted, map[string]bool{"started": true})
}
