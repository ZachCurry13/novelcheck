package api

import (
	"net/http"
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
)

type cacheEntry struct {
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
}

// bump says something changed: every kept answer is out of date.
func (c *respCache) bump() { c.ver.Add(1) }

func (c *respCache) get(key string, ttl time.Duration) (*cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok || e.ver != c.ver.Load() || time.Since(e.at) > ttl {
		return nil, false
	}
	return e, true
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
	if strings.HasPrefix(path, "/api/discover") {
		return discoverTTL
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
		if e, ok := s.rcache.get(key, ttlFor(r.URL.Path)); ok {
			for k, v := range e.header {
				w.Header()[k] = v
			}
			w.Header().Set("X-Cache", "hit")
			w.WriteHeader(e.status)
			_, _ = w.Write(e.body)
			return
		}
		ver := s.rcache.ver.Load() // a change while this ran must not leave the old answer behind
		cap := &capture{ResponseWriter: w}
		w.Header().Set("X-Cache", "miss")
		next.ServeHTTP(cap, r)
		if cap.status == http.StatusOK && len(cap.body) <= 8<<20 {
			h := http.Header{}
			for _, k := range []string{"Content-Type", "Cache-Control", "Etag"} {
				if v := w.Header().Values(k); len(v) > 0 {
					h[k] = v
				}
			}
			s.rcache.put(key, &cacheEntry{at: time.Now(), ver: ver, status: cap.status, header: h, body: cap.body})
		}
	})
}
