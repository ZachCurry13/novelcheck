package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/covers"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Books whose cover Calibre does not have (no cover.jpg, and nothing from Open Library by ISBN) show a drawn
// placeholder. The cover finder looks for a real one in the background (Open Library by ISBN, then a title and
// author search there and in Google Books), slowly and politely, and keeps what it finds. Only a result whose
// title really matches the book is used.

type coverJob struct {
	mu      sync.Mutex
	Running bool `json:"running"`
	stop    bool
	Checked int    `json:"checked"`
	Found   int    `json:"found"`
	Total   int    `json:"total"`
	Current string `json:"current"`
	Note    string `json:"note"`
	Skipped int    `json:"skipped"` // searched in the last week with nothing found, left out this time
}

func (j *coverJob) state() coverJob {
	j.mu.Lock()
	defer j.mu.Unlock()
	return coverJob{Running: j.Running, Checked: j.Checked, Found: j.Found, Total: j.Total, Current: j.Current, Note: j.Note}
}

func (s *Server) coverName(id int64) string { return "book-" + strconv.FormatInt(id, 10) }

// hasCover says whether a book has a real cover somewhere NovelCheck can serve it from, without using the network.
func (s *Server) hasCover(b store.BriefBook) bool {
	copies, _ := s.Store.BookCopies(b.ID)
	if _, _, found := covers.Find(copies, s.insideCalibre); found {
		return true
	}
	if _, _, ok := s.Covers.Stored(s.coverName(b.ID)); ok {
		return true
	}
	return s.Covers.HasOpenLibrary(b.ISBN) || s.Store.AudioItemID(b.ID) != "" || s.Store.DiscoverCoverURL(b.ID) != ""
}

// MissingCovers lists the library books with no real cover.
func (s *Server) MissingCovers() []store.BriefBook {
	books, _ := s.Store.LibraryBooks()
	var out []store.BriefBook
	for _, b := range books {
		if !s.hasCover(b) {
			out = append(out, b)
		}
	}
	return out
}

// findCovers goes through the books with no cover, one every second or so, until done or stopped.
// again (Find covers now, pressed by a person) looks again at books searched in the last week; the daily run
// skips them and says how many it skipped.
func (s *Server) findCovers(again bool) {
	j := &s.covers
	missing := s.MissingCovers()
	j.mu.Lock()
	j.Running, j.stop, j.Checked, j.Found, j.Total, j.Note, j.Skipped = true, false, 0, 0, len(missing), "", 0
	j.mu.Unlock()
	defer func() {
		j.mu.Lock()
		j.Running, j.Current = false, ""
		found, total, note, skipped := j.Found, j.Total, j.Note, j.Skipped
		j.mu.Unlock()
		if total > 0 {
			msg := fmt.Sprintf("Cover finder: found covers for %d of %d books that had none.%s", found, total, note)
			if skipped > 0 {
				msg += fmt.Sprintf(" %d were searched in the last week with nothing found, so they were skipped; Find covers now looks at them again.", skipped)
			}
			s.Store.NotifyRoutine("covers-found", msg, "#/admin?tab=cleanup")
		}
	}()
	ctx := context.Background()
	for _, b := range missing {
		j.mu.Lock()
		stop := j.stop
		j.Current = b.Title
		j.mu.Unlock()
		if stop {
			j.mu.Lock()
			j.Note = " (stopped)"
			j.mu.Unlock()
			return
		}
		if again {
			s.Covers.Forget(s.coverName(b.ID))
		} else if s.Covers.Missed(s.coverName(b.ID)) {
			j.mu.Lock()
			j.Checked++
			j.Skipped++
			j.mu.Unlock()
			continue
		}
		_, _, ok := s.Covers.OpenLibrary(ctx, b.ISBN) // the ISBN's cover, when it has one
		if !ok {
			_, _, ok = s.Covers.Search(ctx, s.coverName(b.ID), b.Title, b.Author)
		}
		j.mu.Lock()
		j.Checked++
		if ok {
			j.Found++
		}
		j.mu.Unlock()
		time.Sleep(900 * time.Millisecond) // be gentle with the places we ask
	}
}

// StartCoverFinder runs the finder unless it already runs; it says whether it started one.
func (s *Server) StartCoverFinder(again bool) bool {
	j := &s.covers
	j.mu.Lock()
	if j.Running {
		j.mu.Unlock()
		return false
	}
	j.Running = true // claimed here, so two clicks do not start two
	j.mu.Unlock()
	go s.findCovers(again)
	return true
}

// CoverLoop looks for missing covers once a day (and a little after start), so new books get one.
func (s *Server) CoverLoop(ctx context.Context) {
	wait := 10 * time.Minute
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = 24 * time.Hour
		s.StartCoverFinder(false)
	}
}

func (s *Server) handleMissingCovers(w http.ResponseWriter, r *http.Request) {
	missing := s.MissingCovers()
	list := missing[:min(len(missing), 50)]
	writeJSON(w, http.StatusOK, map[string]any{"count": len(missing), "books": list, "job": s.covers.state()})
}

func (s *Server) handleFindCovers(w http.ResponseWriter, r *http.Request) {
	if !s.StartCoverFinder(true) {
		writeErr(w, http.StatusConflict, "the cover finder is already running")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"started": true})
}

func (s *Server) handleStopCovers(w http.ResponseWriter, r *http.Request) {
	s.covers.mu.Lock()
	s.covers.stop = true
	s.covers.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
