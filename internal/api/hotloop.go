package api

import (
	"context"
	"net/http/httptest"
	"time"
)

// HotLoop keeps the pages people use worked out: a page asked for in the last half hour is worked out again in the
// background when something has changed since or it is getting old, so the next person to open it gets the answer at
// once. Pages nobody uses are left alone, and it wakes right after any change made through the app. At most a few pages
// per round, one at a time, so a burst of changes never piles work up.
func (s *Server) HotLoop(ctx context.Context) {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-s.rcache.kicker():
			select { // a change often comes in a burst: let it finish first
			case <-time.After(700 * time.Millisecond):
			case <-ctx.Done():
				return
			}
		}
		s.warmHot(8)
	}
}

// warmHot works out again up to max of the pages that were asked for lately and are out of date.
func (s *Server) warmHot(max int) {
	type job struct {
		key string
		e   *cacheEntry
	}
	var due []job
	s.rcache.mu.Lock()
	ver := s.rcache.ver.Load()
	for key, e := range s.rcache.m {
		if e.req == nil || e.next == nil || time.Since(time.Unix(0, e.hit.Load())) > hotFor || s.rcache.busy[key] {
			continue
		}
		if e.ver != ver || time.Since(e.at) > ttlFor(e.req.URL.Path) {
			due = append(due, job{key, e})
		}
	}
	s.rcache.mu.Unlock()
	for i, j := range due {
		if i >= max {
			break
		}
		if !s.rcache.refreshing(j.key, true) {
			continue
		}
		r := j.e.req.Clone(context.WithoutCancel(j.e.req.Context()))
		s.keepAnswer(j.key, httptest.NewRecorder(), r, j.e.next)
		s.rcache.refreshing(j.key, false)
	}
}
