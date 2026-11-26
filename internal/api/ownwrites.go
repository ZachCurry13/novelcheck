package api

import (
	"net/http"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

// ownWrites notes, when a request that changes something has been answered, who made it (the signed-in person), so the
// next page that person opens is worked out afresh and shows their change (respcache.go). Everyone else keeps getting
// the kept answer at once.
func (s *Server) ownWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if !changesData(r.Method, r.URL.Path) { // not a reading ping, a report or a notice marked read
			return
		}
		if u := auth.UserFrom(r); u != nil {
			s.rcache.writes.Store(writerKey(u.ID), time.Now())
		}
	})
}

// kosyncWrites does the same for the Kindle app and the KOReader plugin, which sign in with a name and a sync key.
func (s *Server) kosyncWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if !changesData(r.Method, r.URL.Path) {
			return
		}
		if name := r.Header.Get("x-auth-user"); name != "" {
			if u, err := s.Store.UserByName(name); err == nil {
				s.rcache.writes.Store(writerKey(u.ID), time.Now())
			}
		}
	})
}
