package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The library cleanup lists take seconds to work out on a big library. They are the same for every admin, so
// the server works them out ahead of time and keeps them (respcache.go): opening Library cleanup shows them at
// once. Any change made through the app throws them away; the next minute's round works them out again.

// sharedAnswer is a GET answer that is the same for every admin and slow to work out.
func sharedAnswer(path string) bool {
	switch path {
	case "/api/admin/cleanup", "/api/admin/covers/missing", "/api/admin/same-books":
		return true
	}
	return false
}

// warmPages are the pages kept ready, with the handler that answers each.
func (s *Server) warmPages() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"/api/admin/cleanup":                   s.handleCleanup,
		"/api/admin/covers/missing":            s.handleMissingCovers,
		"/api/admin/covers/missing?limit=2000": s.handleMissingCovers, // Review covers
		"/api/admin/same-books":                s.handleSamePairs,
	}
}

// WarmLoop keeps the cleanup lists worked out: a minute after start, then every minute it works out any that
// were thrown away by a change or are over five minutes old, one at a time.
func (s *Server) WarmLoop(ctx context.Context) {
	wait := time.Minute
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = time.Minute
		s.slowStatus() // the Admin status' slow part is kept up to date too
		admin := s.ownerAdmin()
		if admin == nil {
			continue
		}
		for page, h := range s.warmPages() {
			key := "admins|" + page
			if e, ok, fresh := s.rcache.get(key, ttlFor(strings.SplitN(page, "?", 2)[0])); ok && fresh && e.print == s.libraryPrint() {
				continue
			}
			if !s.rcache.refreshing(key, true) {
				continue
			}
			r := httptest.NewRequest(http.MethodGet, page, nil).WithContext(auth.WithUser(ctx, admin))
			s.keepAnswer(key, httptest.NewRecorder(), r, h)
			s.rcache.refreshing(key, false)
		}
	}
}

// ownerAdmin is the main admin, whose view the kept pages are worked out as.
func (s *Server) ownerAdmin() *store.User {
	admins, _ := s.Store.ListUsersByRole(store.RoleAdmin)
	for i := range admins {
		if admins[i].Owner {
			return &admins[i]
		}
	}
	if len(admins) > 0 {
		return &admins[0]
	}
	return nil
}
