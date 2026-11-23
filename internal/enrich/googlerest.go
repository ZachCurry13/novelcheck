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
// answers meanwhile. The cover finder rests Google the same way (covers/limits.go).

type statusErr struct {
	code int
	msg  string
}

func (e statusErr) Error() string { return e.msg }

var googleRest struct {
	mu    sync.Mutex
	until time.Time
}

func googleResting() bool {
	googleRest.mu.Lock()
	defer googleRest.mu.Unlock()
	return time.Now().Before(googleRest.until)
}

func noteGoogleRefusal(err error) {
	var se statusErr
	if !errors.As(err, &se) || (se.code != http.StatusTooManyRequests && se.code != http.StatusForbidden) {
		return
	}
	now := time.Now().UTC()
	until := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, time.UTC)
	if !until.After(now) {
		until = until.Add(24 * time.Hour)
	}
	googleRest.mu.Lock()
	googleRest.until = until
	googleRest.mu.Unlock()
}
