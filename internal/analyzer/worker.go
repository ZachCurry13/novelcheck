// Package analyzer runs queued book analyses one at a time, honouring the
// admin's token-per-hour cap and scan delay.
package analyzer

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/calibre"
	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Status is reported to the admin dashboard.
type Status struct {
	QueueLength int    `json:"queue_length"`
	CurrentBook int64  `json:"current_book_id"`
	CurrentName string `json:"current_title"`
	State       string `json:"state"` // idle | enriching | analyzing | rate_limited | delaying
	LastError   string `json:"last_error"`
}

type Worker struct {
	Store *store.Store

	mu     sync.Mutex
	queue  []int64
	rerate map[int64]bool // analyzed books being re-rated in place (stay visible meanwhile)
	status Status
	wake   chan struct{}
	cur    *run // the current pass through the queue (Run's goroutine only)
}

func New(st *store.Store) *Worker {
	return &Worker{Store: st, wake: make(chan struct{}, 1), status: Status{State: "idle"}}
}

// Enqueue schedules books for analysis (already marked 'queued' by caller or
// marked here). front=true puts them ahead of any batch (single-book scans).
func (w *Worker) Enqueue(front bool, ids ...int64) {
	for _, id := range ids {
		_ = w.Store.SetStatus(id, "queued", "")
	}
	w.mu.Lock()
	if front {
		w.queue = append(append([]int64{}, ids...), w.queue...)
	} else {
		w.queue = append(w.queue, ids...)
	}
	w.status.QueueLength = len(w.queue)
	w.mu.Unlock()
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Rerate re-analyzes already-rated books (e.g. onto the pepper scale) after
// any other queued work. Their current rating stays in place until the new
// one is saved, so kids don't lose access meanwhile; a failure keeps the old
// rating. Returns how many were added.
func (w *Worker) Rerate(ids ...int64) int {
	w.mu.Lock()
	if w.rerate == nil {
		w.rerate = map[int64]bool{}
	}
	added := 0
	for _, id := range ids {
		if !w.rerate[id] {
			w.rerate[id] = true
			w.queue = append(w.queue, id)
			added++
		}
	}
	w.status.QueueLength = len(w.queue)
	w.mu.Unlock()
	select {
	case w.wake <- struct{}{}:
	default:
	}
	return added
}

// takeRerate reports (and clears) whether id was queued by Rerate.
func (w *Worker) takeRerate(id int64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	r := w.rerate[id]
	delete(w.rerate, id)
	return r
}

// Wipe drops everything waiting and returns queued books to Pending.
func (w *Worker) Wipe() (int64, error) {
	w.mu.Lock()
	w.queue = nil
	w.rerate = nil
	w.status.QueueLength = 0
	w.mu.Unlock()
	return w.Store.ResetQueued()
}

func (w *Worker) Status() Status {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.status
}

func (w *Worker) setState(state string, id int64, title string) {
	w.mu.Lock()
	w.status.State, w.status.CurrentBook, w.status.CurrentName = state, id, title
	w.mu.Unlock()
}

func (w *Worker) pop() (int64, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.queue) == 0 {
		return 0, false
	}
	id := w.queue[0]
	w.queue = w.queue[1:]
	w.status.QueueLength = len(w.queue)
	return id, true
}

// Run processes the queue until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	for {
		id, ok := w.pop()
		if !ok {
			w.runDone()
			w.setState("idle", 0, "")
			select {
			case <-ctx.Done():
				return
			case <-w.wake:
				continue
			}
		}
		if w.takeRerate(id) {
			saved, err := w.rerateOne(ctx, id)
			if err != nil {
				log.Printf("re-rating book %d failed (kept its old rating): %v", id, err)
			}
			w.runBook(saved, err)
		} else {
			saved, err := w.process(ctx, id)
			if err != nil {
				log.Printf("analysis of book %d failed: %v", id, err)
				w.mu.Lock()
				w.status.LastError = fmt.Sprintf("book %d: %v", id, err)
				w.mu.Unlock()
				_ = w.Store.SetStatus(id, "error", err.Error())
				// Same error text is grouped into one notification with a count.
				w.Store.Notify("warning", "analysis", "Rating books is failing: "+err.Error(), "#/system")
			}
			w.runBook(saved, err)
		}
		if ctx.Err() != nil {
			return
		}
		if d := w.Store.SettingInt(store.KeyScanDelaySeconds); d > 0 {
			w.setState("delaying", 0, "")
			sleep(ctx, time.Duration(d)*time.Second)
		}
	}
}

// process rates a queued book; saved reports whether a rating was stored.
func (w *Worker) process(ctx context.Context, id int64) (saved bool, err error) {
	b, err := w.Store.BookByID(id, nil)
	if err != nil {
		return false, nil // deleted meanwhile
	}
	if b.Status != "queued" {
		return false, nil // wiped from the queue after being popped
	}
	_ = w.Store.SetStatus(id, "processing", "")

	a, err := w.rate(ctx, b, func() bool {
		cur, err := w.Store.BookByID(id, nil)
		return err == nil && cur.Status == "processing" // not wiped while waiting
	})
	if err != nil || a == nil {
		return false, err
	}
	return true, w.Store.SaveAnalysis(id, *a)
}

// rate fetches a blurb if needed, waits for token budget, then asks each
// model in order until one gives a valid verdict. still is re-checked after
// waiting; if it returns false, rate gives up quietly (nil, nil).
func (w *Worker) rate(ctx context.Context, b *store.Book, still func() bool) (*store.Analysis, error) {
	id := b.ID
	if b.Blurb == "" {
		w.setState("enriching", id, b.Title)
		ec := enrich.New(w.Store.Setting(store.KeyGoogleBooksAPIKey))
		blurb, _ := ec.Blurb(ctx, b.Title, b.Author, b.ISBN, b.Description)
		b.Blurb = calibre.StripHTML(blurb)
		_ = w.Store.SetBlurb(id, b.Blurb)
	}

	user := llm.UserPrompt(b.Title, b.Author, b.Blurb)
	est := llm.EstimateTokens(llm.SystemPromptFor(w.Store.Setting(store.KeyLanguage))+user) + llm.ExpectedCompletionTokens
	if err := w.waitForBudget(ctx, id, b.Title, est); err != nil {
		return nil, err
	}
	if !still() {
		return nil, nil
	}

	w.setState("analyzing", id, b.Title)
	return w.askAIs(ctx, id, user)
}

// rerateOne re-rates an analyzed book without taking it out of the library:
// its status stays "analyzed" and a failure keeps the old rating.
func (w *Worker) rerateOne(ctx context.Context, id int64) (saved bool, err error) {
	b, err := w.Store.BookByID(id, nil)
	if err != nil || b.Status != "analyzed" || strings.HasPrefix(b.AnalysisModel, "manual:") {
		return false, nil // gone, re-queued normally, or hand-rated by a parent
	}
	a, err := w.rate(ctx, b, func() bool { return true })
	if err != nil || a == nil {
		return false, err
	}
	return true, w.Store.SaveAnalysis(id, *a)
}

// waitForBudget blocks while the trailing-hour token total plus this call's
// estimate would exceed the configured cap (0 = unlimited).
func (w *Worker) waitForBudget(ctx context.Context, id int64, title string, est int) error {
	for waited := false; ; waited = true {
		limit := w.Store.SettingInt(store.KeyTokensPerHour)
		used, err := w.Store.TokensSince("-1 hour")
		if err != nil {
			return err
		}
		if limit <= 0 || used+est <= limit || used == 0 {
			if waited {
				w.Store.Resolve("token-cap")
			}
			return nil
		}
		if !waited {
			w.Store.Notify("warning", "token-cap", tokenCapMessage(limit), "#/admin")
		}
		w.setState("rate_limited", id, title)
		secs, _ := w.Store.SecondsUntilWindowFrees()
		if secs < 15 {
			secs = 15
		}
		if !sleep(ctx, time.Duration(min(secs, 300))*time.Second) {
			return ctx.Err()
		}
	}
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
