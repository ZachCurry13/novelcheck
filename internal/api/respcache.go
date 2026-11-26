package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
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
	maxStale      = 2 * time.Minute  // after a change made by someone else, the kept answer is still shown this long while it is worked out again
	hotFor        = 30 * time.Minute // a page asked for in this time is kept up to date in the background
)

type cacheEntry struct {
	print  string // for the cleanup lists: the library's fingerprint when worked out (libraryPrint)
	at     time.Time
	began  time.Time // when working it out began: a change made before this is in it
	ver    uint64
	hard   uint64        // the access counter when worked out (see bumpHard)
	req    *http.Request // how to work it out again, behind the scenes (kept warm: hotloop.go)
	next   http.Handler
	hit    atomic.Int64 // when it was last asked for (unix nanoseconds)
	status int
	header http.Header
	body   []byte
}

type respCache struct {
	mu   sync.Mutex
	m    map[string]*cacheEntry
	size int
	ver  atomic.Uint64
	// hard counts the changes that decide who may see what (a library made private, a person's rules, a book approved): after
	// one, no kept answer is shown to anyone, as before; after any other change a kept answer is shown while it is refreshed.
	hard atomic.Uint64
	busy map[string]bool // keys being worked out again in the background
	// writes is when each client (a browser's session, a Kindle's sign-in) last changed something. Their next page is worked
	// out afresh so it shows their change; everyone else keeps getting the kept answer at once.
	writes   sync.Map
	kickOnce sync.Once
	kickCh   chan struct{} // wakes the warm-up after a change
}

func (c *respCache) kicker() chan struct{} {
	c.kickOnce.Do(func() { c.kickCh = make(chan struct{}, 1) })
	return c.kickCh
}

// writes is when each person (all their devices) last changed something, kept by their id: see ownWrites.
func writerKey(userID int64) string { return "u|" + strconv.FormatInt(userID, 10) }

func (c *respCache) lastWrite(who string) time.Time {
	if t, ok := c.writes.Load(who); ok {
		return t.(time.Time)
	}
	return time.Time{}
}

// peek gives the kept answer whatever its age or the changes since (the caller decides what is good enough).
func (c *respCache) peek(key string) (*cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	return e, ok
}

// bump says something changed: every kept answer is out of date.
func (c *respCache) bump() { c.ver.Add(1) }

// bumpHard says an access rule changed: no kept answer may be shown.
func (c *respCache) bumpHard() { c.hard.Add(1) }

// sensitiveWrite says whether a change may alter who is allowed to see what.
func sensitiveWrite(path string) bool {
	for _, p := range []string{"/catalogs", "/admin/users", "/users", "/content", "/flags", "/age", "/approve", "/ok", "/rules", "/family", "/sharing", "/prefs", "/permissions", "/areas", "/collections", "/block", "/guide", "/friend", "/role", "/password", "/box-sets"} {
		if strings.Contains(path, p) {
			return true
		}
	}
	return false
}

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

var (
	bookPage         = regexp.MustCompile(`^/api/books/\d+$`)
	ratingInProgress = regexp.MustCompile(`"status":"(pending|queued|processing)"`)
)

// ttlFor is how long a page's answer is kept: Discover's lists change daily, the others change as people use the app.
func ttlFor(path string) time.Duration {
	switch {
	case strings.HasPrefix(path, "/api/discover"):
		return discoverTTL
	case path == "/api/admin/status":
		return 8 * time.Second // the rating worker's progress is on it: only a short while
	case path == "/api/admin/aitools", path == "/api/admin/integrations":
		return 2 * time.Minute // ask other servers, which is slow, and rarely change
	case path == "/api/home", path == "/api/admin/health":
		return 30 * time.Second
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
		client := writerKey(u.ID)
		if e, ok := s.rcache.peek(key); ok && e.hard == s.rcache.hard.Load() {
			// Their own change since it was worked out is not in it: wait for the round already under way, or work it out
			// afresh. Anything else (a change by someone else, a background job, age) is shown at once and worked out
			// again behind it.
			if e.began.Before(s.rcache.lastWrite(client)) {
				e, ok = s.waitForNewer(key, s.rcache.lastWrite(client))
			}
			if ok {
				printOld := e.print != "" && e.print != s.libraryPrint()
				verOld := e.ver != s.rcache.ver.Load()
				age := time.Since(e.at)
				if !((printOld && age >= staleOK) || (verOld && !printOld && age > maxStale)) {
					if (verOld || printOld || age > ttlFor(r.URL.Path)) && s.rcache.refreshing(key, true) {
						go s.refreshAnswer(key, r.Clone(context.WithoutCancel(r.Context())), next)
					}
					e.hit.Store(time.Now().UnixNano())
					for k, v := range e.header {
						w.Header()[k] = v
					}
					w.Header().Set("X-Cache", "hit")
					w.WriteHeader(e.status)
					_, _ = w.Write(e.body)
					return
				}
			}
		}
		w.Header().Set("X-Cache", "miss")
		s.keepAnswer(key, w, r, next)
	})
}

// waitForNewer waits (up to 8 seconds) for the answer being worked out for key to be one that includes a change made at
// since, so the person who made it sees it; false when none arrives (the caller works it out).
func (s *Server) waitForNewer(key string, since time.Time) (*cacheEntry, bool) {
	for end := time.Now().Add(8 * time.Second); time.Now().Before(end); time.Sleep(40 * time.Millisecond) {
		if e, ok := s.rcache.peek(key); ok && !e.began.Before(since) {
			return e, true
		}
		s.rcache.mu.Lock()
		running := s.rcache.busy[key]
		s.rcache.mu.Unlock()
		if !running {
			return nil, false
		}
	}
	return nil, false
}

// keepAnswer works an answer out, sends it to w and keeps it. An answer that says it is still being worked
// on (the AI picking suggestions, a cover search or format job running) changes by itself: not kept.
func (s *Server) keepAnswer(key string, w http.ResponseWriter, r *http.Request, next http.Handler) {
	ver, hard := s.rcache.ver.Load(), s.rcache.hard.Load() // a change while this ran must not leave the old answer behind
	began := time.Now()
	print := ""
	if sharedAnswer(r.URL.Path) {
		print = s.libraryPrint() // a background change to the library (a sync, a join) also makes it old
	}
	cap := &capture{ResponseWriter: w}
	next.ServeHTTP(cap, r)
	moving := bytes.Contains(cap.body, []byte(`"refreshing":true`)) || (sharedAnswer(r.URL.Path) && bytes.Contains(cap.body, []byte(`"running":true`))) ||
		(bookPage.MatchString(r.URL.Path) && ratingInProgress.Match(cap.body)) // a book being rated changes by itself: not kept until done
	if cap.status == http.StatusOK && len(cap.body) <= 8<<20 && !moving {
		h := http.Header{}
		for _, k := range []string{"Content-Type", "Cache-Control", "Etag"} {
			if v := w.Header().Values(k); len(v) > 0 {
				h[k] = v
			}
		}
		e := &cacheEntry{print: print, at: time.Now(), began: began, ver: ver, hard: hard, status: cap.status, header: h, body: cap.body,
			req: r.Clone(context.WithoutCancel(r.Context())), next: next}
		e.hit.Store(time.Now().UnixNano())
		s.rcache.put(key, e)
	}
}

// refreshAnswer works a kept answer out again in the background.
func (s *Server) refreshAnswer(key string, r *http.Request, next http.Handler) {
	defer s.rcache.refreshing(key, false)
	s.keepAnswer(key, httptest.NewRecorder(), r, next)
}
