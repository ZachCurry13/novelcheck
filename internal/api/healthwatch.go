package api

import (
	"context"
	"strings"
	"time"
)

// HealthWatchLoop tells the admins by itself when something on the Health page stays wrong: every ten minutes it looks at
// Health, and an item that has been off for half an hour (Calibre unreachable, the AI stuck, a full disk) gets one notice
// a day, which goes away when the item is fine again. Short blips (Hardcover's rest, a restart) never notify. Kindles,
// genres and covers are left out: they are slow, long-running things the Health page already shows.
func (s *Server) HealthWatchLoop(ctx context.Context) {
	firstBad, told := map[string]time.Time{}, map[string]time.Time{}
	tick := time.NewTicker(10 * time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		s.watchHealthOnce(time.Now(), firstBad, told)
	}
}

func (s *Server) watchHealthOnce(now time.Time, firstBad, told map[string]time.Time) {
	off := map[string]bool{}
	for _, it := range s.healthItems() {
		if it.Level == "ok" || it.ID == "genres" || it.ID == "covers" || strings.HasPrefix(it.ID, "kindle-") {
			continue
		}
		off[it.ID] = true
		if _, ok := firstBad[it.ID]; !ok {
			firstBad[it.ID] = now
		}
		if now.Sub(firstBad[it.ID]) >= 30*time.Minute && now.Sub(told[it.ID]) >= 24*time.Hour {
			told[it.ID] = now
			level := "warning"
			if it.Level == "bad" {
				level = "error"
			}
			s.Store.Notify(level, "health-"+it.ID, it.Title+": "+it.Detail, "#/health")
		}
	}
	for id := range firstBad {
		if !off[id] {
			delete(firstBad, id)
			s.Store.Resolve("health-" + id)
		}
	}
}
