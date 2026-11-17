package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

// A small in-memory cache for the heavy list pages (Library, filters, Discover,
// Collections, reading stats). An answer is kept for a short time per person and
// address, and every change made through the app (any successful POST, PUT,
// PATCH or DELETE) throws all of it away, so a page never shows what someone
// just changed. Changes made by background jobs show up within the time limit.

const (
	cacheTTL      = 45 * time.Second
	discoverTTL   = 15 * time.Minute // Discover's lists change once a day
	maxCacheBytes = 256 << 20
	staleOK       = 15 * time.Minute // a cleanup list the library has changed under is still shown this long
)

type cacheEntry struct {
	print  string // for the cleanup lists: the library's fingerprint when worked out (libraryPrint)
	at     time.Time
	ver    uint64
	status int
	header http.Header
	body   []byte
}

type respCache struct {
	mu   sync.Mutex
	m    map[string]*cacheEntry
	size int
	ver  atomic.Uint64
	busy map[string]bool // keys being worked out again in the background
}

// bump says something changed: every kept answer is out of date.
func (c *respCache) bump() { c.ver.Add(1) }

// get gives the kept answer when nothing was changed through the app since it was made, and says whether it
// is still within its time (an older one is shown at once and worked out again in the background).
func (c *respCache) get(key string, ttl time.Duration) (e *cacheEntry, ok, fresh bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok = c.m[key]
	if !ok || e.ver != c.ver.Load() {
		return nil, false, false
	}
	return e, true, time.Since(e.at) <= ttl
}

// refreshing marks a key as being worked out again; false when it already is.
func (c *respCache) refreshing(key string, on bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.busy == nil {
		c.busy = map[string]bool{}
	}
	if on && c.busy[key] {
		return false
	}
	if on {
		c.busy[key] = true
	} else {
		delete(c.busy, key)
	}
	return true
}

func (c *respCache) put(key string, e *cacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil || c.size+len(e.body) > maxCacheBytes {
		c.m, c.size = map[string]*cacheEntry{}, 0
	}
	if old, ok := c.m[key]; ok {
		c.size -= len(old.body)
	}
	c.m[key] = e
	c.size += len(e.body)
}

type capture struct {
	http.ResponseWriter
	status int
	body   []byte
}

func (c *capture) WriteHeader(code int) {
	if c.status == 0 {
		c.status = code
	}
	c.ResponseWriter.WriteHeader(code)
}

func (c *capture) Write(p []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	c.body = append(c.body, p...)
	return c.ResponseWriter.Write(p)
}

func (c *capture) Unwrap() http.ResponseWriter { return c.ResponseWriter }

// ttlFor is how long a page's answer is kept: Discover's lists change daily, the others change as people use the app.
func ttlFor(path string) time.Duration {
	switch {
	case strings.HasPrefix(path, "/api/discover"):
		return discoverTTL
	case path == "/api/admin/status":
		return 8 * time.Second // the rating worker's progress is on it: only a short while
	case path == "/api/admin/aitools":
		return 2 * time.Minute // asks the AI server, which is slow and rarely changes
	case sharedAnswer(path):
		return 5 * time.Minute // the library cleanup lists: kept ready (warm.go), and thrown away on any change
	}
	return cacheTTL
}

// cached keeps a GET page's answer in memory for a short while (see above).
func (s *Server) cached(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r)
		if u == nil || r.Method != http.MethodGet || r.URL.Query().Get("fresh") != "" {
			next.ServeHTTP(w, r)
			return
		}
		key := strconv.FormatInt(u.ID, 10) + "|" + r.URL.RequestURI()
		if sharedAnswer(r.URL.Path) {
			key = "admins|" + r.URL.RequestURI() // the same for every admin, so one kept answer serves them all
		}
		// The cleanup lists: when only a background pass changed the library since (a title suggested, a book
		// added), the kept answer is still shown at once (up to 15 minutes old) and worked out again behind it,
		// instead of making whoever opens the page wait for it.
		if e, ok, fresh := s.rcache.get(key, ttlFor(r.URL.Path)); ok && (e.print == "" || e.print == s.libraryPrint() || time.Since(e.at) < staleOK) {
			if (!fresh || (e.print != "" && e.print != s.libraryPrint())) && s.rcache.refreshing(key, true) { // shown now, worked out again behind it
				go s.refreshAnswer(key, r.Clone(context.WithoutCancel(r.Context())), next)
			}
			for k, v := range e.header {
				w.Header()[k] = v
			}
			w.Header().Set("X-Cache", "hit")
			w.WriteHeader(e.status)
			_, _ = w.Write(e.body)
			return
		}
		w.Header().Set("X-Cache", "miss")
		s.keepAnswer(key, w, r, next)
	})
}

// keepAnswer works an answer out, sends it to w and keeps it. An answer that says it is still being worked
// on (the AI picking suggestions, a cover search or format job running) changes by itself: not kept.
func (s *Server) keepAnswer(key string, w http.ResponseWriter, r *http.Request, next http.Handler) {
	ver := s.rcache.ver.Load() // a change while this ran must not leave the old answer behind
	print := ""
	if sharedAnswer(r.URL.Path) {
		print = s.libraryPrint() // a background change to the library (a sync, a join) also makes it old
	}
	cap := &capture{ResponseWriter: w}
	next.ServeHTTP(cap, r)
	moving := bytes.Contains(cap.body, []byte(`"refreshing":true`)) || (sharedAnswer(r.URL.Path) && bytes.Contains(cap.body, []byte(`"running":true`)))
	if cap.status == http.StatusOK && len(cap.body) <= 8<<20 && !moving {
		h := http.Header{}
		for _, k := range []string{"Content-Type", "Cache-Control", "Etag"} {
			if v := w.Header().Values(k); len(v) > 0 {
				h[k] = v
			}
		}
		s.rcache.put(key, &cacheEntry{print: print, at: time.Now(), ver: ver, status: cap.status, header: h, body: cap.body})
	}
}

// refreshAnswer works a kept answer out again in the background.
func (s *Server) refreshAnswer(key string, r *http.Request, next http.Handler) {
	defer s.rcache.refreshing(key, false)
	s.keepAnswer(key, httptest.NewRecorder(), r, next)
}
