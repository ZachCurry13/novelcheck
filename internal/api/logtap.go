package api

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Background work (Calibre syncs, rating, Deep Scans, Hardcover, covers, names, events, Audiobookshelf) reports
// its problems in the server's log, which nobody sees. LogWriter passes every log line on and also keeps each
// line that reports a failure in the bug report's error list (code BG-xxxx), so every error shows up there by
// itself. The same failure is kept once every ten minutes, so one stuck job can't fill the list.

var failure = regexp.MustCompile(`(?i)\b(error|errors:? [1-9]|failed|failure|fails|panic|refused|timed out|timeout|cannot|can't|couldn't|unable|denied|not saved|stopped by a problem)\b`)

// quiet are log lines that mention a failure word without being one.
var quiet = regexp.MustCompile(`(?i)(errors?: 0\b|0 errors|no errors|without errors)`)

type logTap struct {
	next io.Writer
	st   *store.Store
	mu   sync.Mutex
	seen map[string]time.Time
	part []byte
}

// LogWriter is where the server's log goes: next (the console) and, for failures, the bug report.
func (s *Server) LogWriter(next io.Writer) io.Writer {
	return &logTap{next: next, st: s.Store, seen: map[string]time.Time{}}
}

func (t *logTap) Write(p []byte) (int, error) {
	n, err := t.next.Write(p)
	t.mu.Lock()
	t.part = append(t.part, p...)
	var lines []string
	for {
		i := strings.IndexByte(string(t.part), '\n')
		if i < 0 {
			break
		}
		lines = append(lines, string(t.part[:i]))
		t.part = t.part[i+1:]
	}
	t.mu.Unlock()
	for _, l := range lines {
		t.keep(l)
	}
	return n, err
}

func (t *logTap) keep(line string) {
	// "2026/11/17 10:01:02 hardcover ratings: 429 Too Many Requests" → where = "hardcover ratings"
	msg := strings.TrimSpace(line)
	if len(msg) > 20 && msg[4] == '/' && msg[7] == '/' {
		msg = strings.TrimSpace(msg[20:])
	}
	if msg == "" || !failure.MatchString(msg) || quiet.MatchString(msg) || strings.Contains(msg, `" from `) { // request lines are logged apart
		return
	}
	where := "background"
	if i := strings.Index(msg, ": "); i > 0 && i < 40 {
		where = msg[:i]
	}
	key := where + "|" + strings.Map(func(r rune) rune { // the same failure with other numbers in it
		if r >= '0' && r <= '9' {
			return '#'
		}
		return r
	}, msg)
	t.mu.Lock()
	if at, ok := t.seen[key]; ok && time.Since(at) < 10*time.Minute {
		t.mu.Unlock()
		return
	}
	t.seen[key] = time.Now()
	if len(t.seen) > 2000 {
		t.seen = map[string]time.Time{}
	}
	t.mu.Unlock()
	b := make([]byte, 2)
	_, _ = rand.Read(b)
	t.st.LogError(store.ServerError{Code: "BG-" + hex.EncodeToString(b), Method: "background", Path: where, Detail: msg})
}
