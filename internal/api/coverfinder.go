package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/covers"
	"github.com/zachcurry13/novelcheck/internal/hardcover"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Books whose cover Calibre does not have (no cover.jpg, and nothing from Open Library by ISBN) show a drawn
// placeholder. The cover finder looks for a real one in the background (Open Library by ISBN, then Hardcover
// (hccover.go), then a title and author search in Open Library and Google Books), slowly and politely, and keeps what it finds. Only a result whose
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
	Free    int    `json:"free"`    // how many of Total are free books (events), looked for after the library's
	Retry   int    `json:"retry"`   // not found while a place refused to answer (Google's daily limit): looked at again next run
}

func (j *coverJob) state() coverJob {
	j.mu.Lock()
	defer j.mu.Unlock()
	return coverJob{Running: j.Running, Checked: j.Checked, Found: j.Found, Total: j.Total, Current: j.Current, Note: j.Note, Free: j.Free, Retry: j.Retry}
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
	// An audiobook's square cover from Audiobookshelf doesn't count: the finder still looks for the book-shaped one.
	return s.Covers.HasOpenLibrary(b.ISBN) || s.Store.DiscoverCoverURL(b.ID) != ""
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

// freeCoverLimit is how many free books one run looks at, newest first (a Stuff Your Kindle day lists up to a
// thousand); the next run takes the following ones.
const freeCoverLimit = 250

// MissingFreeCovers lists the free books (on a running event, not in a library) with no cover, newest first.
func (s *Server) MissingFreeCovers(limit int) []store.BriefBook {
	books, _ := s.Store.FreeBooks()
	var out []store.BriefBook
	for _, b := range books {
		if len(out) < limit && !s.hasCover(b) {
			out = append(out, b)
		}
	}
	return out
}

// isbn10 is an ASIN that is really a print book's ISBN-10 ("" for a Kindle ASIN like B00ABC1234).
func isbn10(asin string) string {
	a := strings.ToUpper(strings.TrimSpace(asin))
	if len(a) != 10 || strings.Trim(a[:9], "0123456789") != "" || strings.Trim(a[9:], "0123456789X") != "" {
		return ""
	}
	return a
}

// findCovers goes through the books with no cover, one every second or so, until done or stopped: the
// library's, then the free books of the running events (freeOnly: only those, for the quick pass that gives new
// free books a cover soon). again (Find covers now, pressed by a person) looks again at books searched in the
// last week; the daily run skips them and says how many it skipped.
func (s *Server) findCovers(again, freeOnly bool) {
	j := &s.covers
	defer func() { s.missing.mu.Lock(); s.missing.ok = false; s.missing.mu.Unlock() }() // covers were found: the missing list is new
	var missing []store.BriefBook
	if !freeOnly {
		missing = s.MissingCovers()
	}
	free := s.MissingFreeCovers(freeCoverLimit)
	missing = append(missing, free...)
	j.mu.Lock()
	j.Running, j.stop, j.Checked, j.Found, j.Total, j.Note, j.Skipped, j.Free, j.Retry = true, false, 0, 0, len(missing), "", 0, len(free), 0
	j.mu.Unlock()
	defer func() {
		j.mu.Lock()
		j.Running, j.Current = false, ""
		found, total, note, skipped, nfree, retry := j.Found, j.Total, j.Note, j.Skipped, j.Free, j.Retry
		j.mu.Unlock()
		if total > 0 && (!freeOnly || found > 0) { // the quick pass only says something when it found one
			msg := fmt.Sprintf("Cover finder: found covers for %d of %d books that had none.%s", found, total, note)
			if nfree > 0 {
				msg += fmt.Sprintf(" %d of them were free books.", nfree)
			}
			if retry > 0 {
				msg += fmt.Sprintf(" Google Books refused %d of them (its daily limit was used up), so they are looked at again on the next run.", retry)
			}
			if skipped > 0 {
				msg += fmt.Sprintf(" %d were searched in the last week with nothing found, so they were skipped; Find covers now looks at them again.", skipped)
			}
			if found < total {
				s.cleanupNote("Cover finder", fmt.Sprintf("no cover found for %d of %d books (%d skipped as searched this week, %d refused by Google). Some: %s", total-found, total, skipped, retry, s.coverMisses(missing, 25)))
			}
			s.Store.NotifyRoutine("covers-found", msg, "#/admin?tab=cleanup")
		}
	}()
	ctx := context.Background()
	var hc *hardcover.Client // Hardcover as a cover source, when someone in the family connected it
	if token := s.Store.RatingsToken(); token != "" {
		hc = &hardcover.Client{Token: token, URL: hardcoverURL}
	}
	cv := s.Covers
	cv.NoGoogle = freeOnly // the quick pass for free books leaves Google's small daily limit to the library
	var backup map[string]string
	if !freeOnly {
		backup = s.kindleBackupFiles()
	}
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
		if s.adoptOldCover(b.ID) { // the book was deleted and came back: its cover was kept
			j.mu.Lock()
			j.Checked++
			j.Found++
			j.mu.Unlock()
			continue
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
		ok, refused := s.findOneCover(ctx, cv, hc, b, backup)
		j.mu.Lock()
		if !ok && refused {
			j.Retry++
		}
		j.Checked++
		if ok {
			j.Found++
		}
		j.mu.Unlock()
		time.Sleep(900 * time.Millisecond) // be gentle with the places we ask
	}
}

// StartCoverFinder runs the finder unless it already runs; it says whether it started one.
func (s *Server) StartCoverFinder(again bool) bool { return s.startCovers(again, false) }

func (s *Server) startCovers(again, freeOnly bool) bool {
	j := &s.covers
	j.mu.Lock()
	if j.Running {
		j.mu.Unlock()
		return false
	}
	j.Running = true // claimed here, so two clicks do not start two
	j.mu.Unlock()
	go s.findCovers(again, freeOnly)
	return true
}

// CoverLoop looks for missing covers once a day (and a little after start), so new books get one, and for the
// free books of the running events every three hours (the events are checked at that pace), so a free book has a
// cover while it is still free.
func (s *Server) CoverLoop(ctx context.Context) {
	wait := 10 * time.Minute
	daily := time.Now().Add(wait)
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		if time.Now().Before(daily) { // a three-hourly pass
			s.startCovers(false, true)
		} else {
			daily = time.Now().Add(24 * time.Hour)
			s.StartCoverFinder(false)
		}
		wait = 3 * time.Hour
	}
}

// missingCache keeps the books-with-no-cover list for a minute: working it out looks at every book's files, and
// the page asks for it again and again while the cover finder runs.
type missingCache struct {
	mu   sync.Mutex
	list []store.BriefBook
	at   time.Time
	ok   bool
	ver  uint64
}

func (s *Server) missingForPage() []store.BriefBook {
	c := &s.missing
	c.mu.Lock()
	defer c.mu.Unlock()
	if v := s.rcache.ver.Load(); !c.ok || c.ver != v || time.Since(c.at) > time.Minute {
		c.list, c.at, c.ok, c.ver = s.MissingCovers(), time.Now(), true, v
	}
	return c.list
}

func (s *Server) handleMissingCovers(w http.ResponseWriter, r *http.Request) {
	missing := s.missingForPage()
	limit := 50
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 2000 {
		limit = n // Review covers goes through them all
	}
	list := missing[:min(len(missing), limit)]
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

// coverMisses names up to n of the books still without a cover after a run (title · author · ISBN), for the cleanup log.
func (s *Server) coverMisses(books []store.BriefBook, n int) string {
	var out []string
	for _, b := range books {
		if len(out) >= n {
			break
		}
		if !s.hasCover(b) {
			out = append(out, b.Title+" · "+b.Author+" · "+orNone(b.ISBN))
		}
	}
	return strings.Join(out, " | ")
}
