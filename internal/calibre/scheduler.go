package calibre

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/safe"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Syncer serialises Calibre syncs and runs them on the configured schedule.
type Syncer struct {
	Store *store.Store
	Dir   string // container mount point (NOVELCHECK_CALIBRE_DIR)
	// NewBooks, when set, runs after a sync added books and reports whether
	// they will be rated automatically (it wakes the rating worker).
	NewBooks func() bool
	mu       sync.Mutex
}

// Available reports whether the selected folder is a Calibre library.
func (s *Syncer) Available() bool {
	return HasLibrary(s.LibraryDir())
}

// Run performs one sync and records the outcome in settings.
func (s *Syncer) Run() (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	start := time.Now()
	res, err := Sync(s.Store, s.LibraryDir())
	if err == nil {
		s.Store.TidySame() // a book added to a list before it was in the library joins its new copy
	}
	summary := map[string]any{"at": time.Now().UTC().Format(time.RFC3339), "result": res}
	if err != nil {
		summary["error"] = err.Error()
		s.Store.Notify("error", "calibre-sync", "Calibre sync failed: "+err.Error(), "#/admin")
	} else {
		s.Store.Resolve("calibre-sync")
		if n := s.Store.BooksCreatedSince(start); n > 0 {
			next := "They're waiting to be rated."
			if s.NewBooks != nil && s.NewBooks() {
				next = "They'll be rated automatically."
			}
			s.Store.NotifyRoutine("calibre-new", fmt.Sprintf("Calibre sync added %d new book%s. %s",
				n, map[bool]string{true: "", false: "s"}[n == 1], next), "#/library?spice=Pending")
		}
	}
	b, _ := json.Marshal(summary)
	_ = s.Store.SetSetting(store.KeyCalibreLastSync, start.UTC().Format(time.RFC3339)) // when it began: a change made during the sync still counts as new
	_ = s.Store.SetSetting(store.KeyCalibreLastResult, string(b))
	return res, err
}

// Due reports whether the polling interval has elapsed. An interval of 0
// means manual trigger only.
func (s *Syncer) Due(now time.Time) bool {
	hours := s.Store.SettingInt(store.KeyCalibrePollHours)
	if hours <= 0 {
		return false
	}
	last, err := time.Parse(time.RFC3339, s.Store.Setting(store.KeyCalibreLastSync))
	if err != nil {
		return true
	}
	return now.Sub(last) >= time.Duration(hours)*time.Hour
}

// Changed reports whether Calibre's database was written (a book added in Calibre-Web, say)
// since the last sync and has been quiet for a little while. With a schedule on, that starts a
// sync at once instead of waiting for the next one.
func (s *Syncer) Changed(now time.Time) bool {
	if s.Store.SettingInt(store.KeyCalibrePollHours) <= 0 {
		return false // manual only
	}
	last, err := time.Parse(time.RFC3339, s.Store.Setting(store.KeyCalibreLastSync))
	if err != nil {
		return false
	}
	dir := s.LibraryDir()
	var newest time.Time
	for _, f := range []string{"metadata.db", "metadata.db-wal"} {
		if info, err := os.Stat(filepath.Join(dir, f)); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest.After(last) && now.Sub(newest) >= 20*time.Second
}

// Loop checks once a minute whether a scheduled sync is due.
func (s *Syncer) Loop(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		if s.Available() && (s.Due(time.Now()) || s.Changed(time.Now())) {
			var res Result
			// A panic in one sync is logged; the schedule (and the app) keep going.
			if err := safe.Run("calibre sync", func() (err error) { res, err = s.Run(); return err }); err != nil {
				log.Printf("calibre sync failed: %v", err)
			} else {
				log.Printf("calibre sync: %d books, %d files, %d removed", res.Books, res.Copies, res.Removed)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
