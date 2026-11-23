package enrich

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

// Google Books allows a set number of lookups a day per key. Once it says the day's limit is used up (429, or 403
// for a key over its limit), the blurb lookups leave Google alone until the limit starts again (midnight in
// California, 08:00 UTC) instead of asking it for every book and waiting on each refusal. Open Library still
// answers meanwhile. The cover finder shares this pause (covers/search.go).

type statusErr struct {
	code int
	msg  string
}

func (e statusErr) Error() string { return e.msg }

var googleRest struct {
	mu    sync.Mutex
	until time.Time
}

// GoogleResting says whether Google Books said its daily limit is used up and it hasn't started again yet.
func GoogleResting() bool {
	googleRest.mu.Lock()
	defer googleRest.mu.Unlock()
	return time.Now().Before(googleRest.until)
}

func noteGoogleRefusal(err error) {
	var se statusErr
	if errors.As(err, &se) && (se.code == http.StatusTooManyRequests || se.code == http.StatusForbidden) {
		RestGoogle()
	}
}

// RestGoogle leaves Google Books alone until its daily limit starts again (08:00 UTC).
func RestGoogle() {
	now := time.Now().UTC()
	until := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, time.UTC)
	if !until.After(now) {
		until = until.Add(24 * time.Hour)
	}
	googleRest.mu.Lock()
	googleRest.until = until
	googleRest.mu.Unlock()
}

// ResumeGoogle ends a pause (tests).
func ResumeGoogle() {
	googleRest.mu.Lock()
	googleRest.until = time.Time{}
	googleRest.mu.Unlock()
}
