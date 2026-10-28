package api

import (
	"net/http"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

// handleMyReadingStats sends someone's own reading for the Reading stats page
// (see readingStatsFor).
func (s *Server) handleMyReadingStats(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	s.Store.RematchUnmatched(u.ID)
	s.backfillHours(u.ID)
	writeJSON(w, http.StatusOK, s.readingStatsFor(u, u))
}

// backfillHours reads the statistics file already in someone's folder: once
// per run (a new version may keep more of it, like highlights), and again,
// at most every ten minutes, while no hours are stored. It was uploaded
// before NovelCheck kept the hours, and KOReader only uploads when something changed.
func (s *Server) backfillHours(userID int64) {
	has := s.Store.HasReadingHours(userID)
	if _, done := s.hoursSeen.Load(userID); done && has {
		return
	}
	if !has {
		if at, ok := s.hoursTried.Load(userID); ok && time.Since(at.(time.Time)) < 10*time.Minute {
			return
		}
	}
	s.hoursSeen.Store(userID, true)
	if path, _ := s.statsPath(userID); path != "" {
		s.importKOStats(userID)
	}
	if !s.Store.HasReadingHours(userID) {
		s.hoursTried.Store(userID, time.Now())
	}
}
