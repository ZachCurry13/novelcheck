package calibre

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Syncer serialises Calibre syncs and runs them on the configured schedule.
type Syncer struct {
	Store *store.Store
	Dir   string
	mu    sync.Mutex
}

// Available reports whether a Calibre library is mounted.
func (s *Syncer) Available() bool {
	_, err := os.Stat(filepath.Join(s.Dir, "metadata.db"))
	return err == nil
}

// Run performs one sync and records the outcome in settings.
func (s *Syncer) Run() (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := Sync(s.Store, s.Dir)
	summary := map[string]any{"at": time.Now().UTC().Format(time.RFC3339), "result": res}
	if err != nil {
		summary["error"] = err.Error()
	}
	b, _ := json.Marshal(summary)
	_ = s.Store.SetSetting(store.KeyCalibreLastSync, time.Now().UTC().Format(time.RFC3339))
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

// Loop checks once a minute whether a scheduled sync is due.
func (s *Syncer) Loop(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		if s.Available() && s.Due(time.Now()) {
			if res, err := s.Run(); err != nil {
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
