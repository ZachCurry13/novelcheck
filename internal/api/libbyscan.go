package api

import (
	"context"
	"net/http"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/libby"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Every book in the library that the person does not have as an audiobook is looked up, in the background and
// slowly, at each of their Libby libraries. What is found is kept, so the Library can show "🎧 at your library" and
// filter by it without asking the library service again. Found ones are checked again every 3 days (availability
// changes), ones the libraries do not lend every 14.

const libbyScanBatch = 400

// scanLibbyAudio checks the books that are due for one person; it says how many it looked at.
func (s *Server) scanLibbyAudio(ctx context.Context, userID int64, max int, pause time.Duration) int {
	libs := s.Store.LibbyLibraries(userID)
	if len(libs) == 0 {
		return 0
	}
	checked, failures := 0, 0
	for _, b := range s.Store.LibbyAudioDue(userID, max) {
		if ctx.Err() != nil {
			break
		}
		var best libby.Availability
		asked := false
		for _, l := range libs {
			au, err := s.Libby.CheckMedia(ctx, l.Key, b.Title, b.Author, "audiobook")
			if err != nil {
				continue
			}
			asked = true
			au.Library = l.Name
			if better(au, best) {
				best = au
			}
		}
		if !asked { // the library service is not answering: stop, try again at the next run
			failures++
			if failures >= 3 {
				break
			}
			continue
		}
		failures = 0
		_ = s.Store.SaveLibbyAudio(userID, store.LibbyAudioRow{BookID: b.ID, Found: best.Found, Available: best.Available, Holds: best.Holds,
			Copies: best.Copies, WaitDays: best.WaitDays, URL: best.URL, Library: best.Library})
		checked++
		time.Sleep(pause)
	}
	return checked
}

// LibbyScanLoop runs the check for everyone who has a library, every 6 hours.
func (s *Server) LibbyScanLoop(ctx context.Context) {
	wait := 15 * time.Minute
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = 6 * time.Hour
		for _, id := range s.Store.UsersWithLibby() {
			s.scanLibbyAudio(ctx, id, libbyScanBatch, 500*time.Millisecond)
		}
	}
}

// handleLibbyScan: POST /api/me/libby/scan checks this person's books now (in the background; wait=1 waits, for small libraries).
func (s *Server) handleLibbyScan(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	if len(s.Store.LibbyLibraries(u.ID)) == 0 {
		writeErr(w, http.StatusBadRequest, "add a library first")
		return
	}
	if r.URL.Query().Get("wait") == "1" {
		n := s.scanLibbyAudio(r.Context(), u.ID, libbyScanBatch, 0)
		writeJSON(w, http.StatusOK, map[string]int{"checked": n})
		return
	}
	go s.scanLibbyAudio(context.Background(), u.ID, libbyScanBatch, 500*time.Millisecond)
	writeJSON(w, http.StatusAccepted, map[string]bool{"started": true})
}
