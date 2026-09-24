package analyzer

import (
	"fmt"
	"strconv"
	"time"
)

// run is one pass through the queue: from the first book after the worker
// was idle until the queue is empty again. It feeds the "rating finished"
// notice (books rated and failed, tokens, cost).
type run struct {
	start         time.Time
	rated, failed int
}

// minBatchNotice skips the notice for a single "Analyze now", which the
// person is already watching.
const minBatchNotice = 2

func (w *Worker) runBook(saved bool, err error) {
	if w.cur == nil {
		w.cur = &run{start: time.Now()}
	}
	switch {
	case err != nil:
		w.cur.failed++
	case saved:
		w.cur.rated++
	}
}

// runDone posts the summary when the queue empties, then forgets the run.
func (w *Worker) runDone() {
	r := w.cur
	w.cur = nil
	if r == nil || r.rated+r.failed < minBatchNotice {
		return
	}
	tokens, cost := w.Store.UsageSince(r.start)
	w.Store.NotifyRoutine("batch-done", batchSummary(r.rated, r.failed, tokens, cost), "#/usage")
}

// batchSummary reads like "Rating finished: 18 books rated, 2 failed ·
// 18,400 tokens · $0.0123" (no cost for a free local AI).
func batchSummary(rated, failed, tokens int, cost float64) string {
	msg := fmt.Sprintf("Rating finished: %d book%s rated", rated, plural(rated))
	if failed > 0 {
		msg += fmt.Sprintf(", %d failed", failed)
	}
	msg += " · " + thousands(tokens) + " tokens"
	if cost > 0 {
		msg += fmt.Sprintf(" · $%.4f", cost)
	}
	return msg
}

// tokenCapMessage explains a pause for the hourly token limit.
func tokenCapMessage(limit int) string {
	return "Hourly AI limit reached (" + thousands(limit) + " tokens). Rating is paused and will continue by itself."
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// thousands formats 18400 as "18,400".
func thousands(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0 && s[i-1] != '-'; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
