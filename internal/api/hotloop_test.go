package api

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// A page that was asked for lately and is out of date (something changed) is worked out again by the warm-up; one nobody
// asked for in half an hour is left alone.
func TestWarmHotWorksOutUsedPagesOnly(t *testing.T) {
	s := &Server{}
	var calls atomic.Int32
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	req := httptest.NewRequest(http.MethodGet, "/api/home", nil)
	mk := func(lastUsed time.Duration) *cacheEntry {
		e := &cacheEntry{at: time.Now(), began: time.Now(), ver: s.rcache.ver.Load(), req: req, next: next, status: 200, body: []byte(`old`)}
		e.hit.Store(time.Now().Add(-lastUsed).UnixNano())
		return e
	}
	s.rcache.put("1|/api/home", mk(time.Minute))
	s.rcache.put("2|/api/home", mk(2*time.Hour)) // not used for hours
	s.warmHot(8)
	if calls.Load() != 0 {
		t.Fatalf("nothing changed, nothing to do: %d", calls.Load())
	}
	s.rcache.bump() // a change
	s.warmHot(8)
	if calls.Load() != 1 {
		t.Fatalf("only the page in use is worked out again: %d", calls.Load())
	}
	e, _ := s.rcache.peek("1|/api/home")
	if string(e.body) != `{"ok":true}` || e.ver != s.rcache.ver.Load() {
		t.Fatalf("the kept answer is new: %q ver %d", e.body, e.ver)
	}
	if old, _ := s.rcache.peek("2|/api/home"); string(old.body) != "old" {
		t.Fatal("the unused page was left alone")
	}
}
