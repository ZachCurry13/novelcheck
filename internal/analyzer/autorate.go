package analyzer

import (
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // the admin's time zone for quiet hours, even in a bare container

	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Automatic rating: when the worker runs out of books it takes the next
// waiting ones itself (Up Next and wishlist books first, then the newest).
// It's on by default only with a free, local AI, can be limited to set
// hours, and still keeps to the hourly token limit and the scan delay.

// feedEvery is how often an idle worker looks for waiting books.
const feedEvery = time.Minute

// maxFeed caps one automatic batch when "books per batch" is 0 (all).
const maxFeed = 50

// AutoRateOn reports whether automatic rating is on: the admin's choice,
// or by default whether the main AI runs on the home network.
func AutoRateOn(st *store.Store) bool {
	switch st.Setting(store.KeyAutoRate) {
	case "on":
		return true
	case "off":
		return false
	}
	ais := st.AIConfigs()
	return len(ais) > 0 && ais[0].Provider != "anthropic" && llm.IsLocal(ais[0].BaseURL)
}

// ParseHours reads "23-7" (from 23:00 until 07:00). ok is false for "" or
// anything else, meaning any time.
func ParseHours(v string) (from, to int, ok bool) {
	a, b, found := strings.Cut(strings.TrimSpace(v), "-")
	if !found {
		return 0, 0, false
	}
	from, err1 := strconv.Atoi(a)
	to, err2 := strconv.Atoi(b)
	if err1 != nil || err2 != nil || from < 0 || from > 23 || to < 0 || to > 23 || from == to {
		return 0, 0, false
	}
	return from, to, true
}

// RateHoursOpen reports whether automatic rating may run at now: always,
// unless hours are set (in the admin's time zone).
func RateHoursOpen(st *store.Store, now time.Time) bool {
	from, to, ok := ParseHours(st.Setting(store.KeyAutoRateHours))
	if !ok {
		return true
	}
	loc := time.UTC
	if tz := st.Setting(store.KeyAutoRateTZ); tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	h := now.In(loc).Hour()
	if from < to {
		return h >= from && h < to
	}
	return h >= from || h < to // across midnight, e.g. 23-7
}

// feed queues the next waiting books when automatic rating allows it and
// says how many.
func (w *Worker) feed(now time.Time) int {
	if !AutoRateOn(w.Store) || !RateHoursOpen(w.Store, now) {
		return 0
	}
	n := w.Store.SettingInt(store.KeyBatchSize)
	if n <= 0 || n > maxFeed {
		n = maxFeed
	}
	ids, err := w.Store.QueueForAnalysis(n)
	if err != nil || len(ids) == 0 {
		return 0
	}
	w.Enqueue(false, ids...)
	return len(ids)
}

// Kick wakes an idle worker so it looks for waiting books now (after a
// Calibre sync or an import) instead of within the minute.
func (w *Worker) Kick() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}
