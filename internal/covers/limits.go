package covers

import (
	"sync"
	"time"
)

// Limits remembers that Google Books said its daily limit is used up, so the cover searches leave Google alone
// until the limit starts again (midnight in California, 08:00 UTC) instead of asking it hundreds of times more
// and taking every refusal for "no cover". A nil Limits remembers nothing.
type Limits struct {
	mu          sync.Mutex
	googleUntil time.Time
}

func (l *Limits) googleResting() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return time.Now().Before(l.googleUntil)
}

func (l *Limits) restGoogle() {
	if l == nil {
		return
	}
	now := time.Now().UTC()
	until := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, time.UTC)
	if !until.After(now) {
		until = until.Add(24 * time.Hour)
	}
	l.mu.Lock()
	l.googleUntil = until
	l.mu.Unlock()
}

// GoogleResting says whether Google Books refused for the day (its daily limit is used up).
func (c Cache) GoogleResting() bool { return c.Limits.googleResting() }
