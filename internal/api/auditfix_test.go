package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/config"
)

// A device name of ".." or "." stays inside the person's own backup folder.
func TestKindleDirStaysInside(t *testing.T) {
	s := &Server{Cfg: config.Config{DataDir: "/data"}}
	for _, d := range []string{"..", ".", "", "a/../.."} {
		if got := s.kindleDir(1, d); filepath.Dir(got) != filepath.Join("/data", "kindle-backup", "1") {
			t.Errorf("device %q: %s", d, got)
		}
	}
}

// The visitor's address comes from the proxy headers only when the connection is from this machine (the tunnel).
func TestRealIPTrustsOnlyLoopback(t *testing.T) {
	var seen string
	h := realIP(true)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = remoteHost(r) }))
	for peer, want := range map[string]string{"127.0.0.1:5000": "203.0.113.9", "192.168.1.50:5000": "192.168.1.50"} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = peer
		r.Header.Set("CF-Connecting-IP", "203.0.113.9")
		h.ServeHTTP(httptest.NewRecorder(), r)
		if seen != want {
			t.Errorf("from %s: got %s, want %s", peer, seen, want)
		}
	}
}

// Old wrong-code times are dropped, so the list stays small.
func TestGuessLimiterForgetsOldTries(t *testing.T) {
	l := &loginLimiter{attempts: map[string][]time.Time{"x": {time.Now().Add(-time.Hour), time.Now().Add(-time.Hour)}}, max: 2, window: time.Minute}
	l.fail("x")
	if len(l.attempts["x"]) != 1 || l.blocked("x") {
		t.Fatalf("old tries kept: %v", l.attempts["x"])
	}
}
