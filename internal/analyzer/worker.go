// Package analyzer runs queued book analyses one at a time, honouring the
// admin's token-per-hour cap and scan delay.
package analyzer

import (
	"context"
	"fmt"
	"log"
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
	status Status
	wake   chan struct{}
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

// Wipe drops everything waiting and returns queued books to Pending.
func (w *Worker) Wipe() (int64, error) {
	w.mu.Lock()
	w.queue = nil
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
			w.setState("idle", 0, "")
			select {
			case <-ctx.Done():
				return
			case <-w.wake:
				continue
			}
		}
		if err := w.process(ctx, id); err != nil {
			log.Printf("analysis of book %d failed: %v", id, err)
			w.mu.Lock()
			w.status.LastError = fmt.Sprintf("book %d: %v", id, err)
			w.mu.Unlock()
			_ = w.Store.SetStatus(id, "error", err.Error())
			// Same error text is grouped into one notification with a count.
			w.Store.Notify("warning", "analysis", "Rating books is failing: "+err.Error(), "#/system")
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

func (w *Worker) process(ctx context.Context, id int64) error {
	b, err := w.Store.BookByID(id, nil)
	if err != nil {
		return nil // deleted meanwhile
	}
	if b.Status != "queued" {
		return nil // wiped from the queue after being popped
	}
	_ = w.Store.SetStatus(id, "processing", "")

	if b.Blurb == "" {
		w.setState("enriching", id, b.Title)
		ec := enrich.New(w.Store.Setting(store.KeyGoogleBooksAPIKey))
		blurb, _ := ec.Blurb(ctx, b.Title, b.Author, b.ISBN, b.Description)
		b.Blurb = calibre.StripHTML(blurb)
		_ = w.Store.SetBlurb(id, b.Blurb)
	}

	user := llm.UserPrompt(b.Title, b.Author, b.Blurb)
	est := llm.EstimateTokens(llm.SystemPrompt+user) + llm.ExpectedCompletionTokens
	if err := w.waitForBudget(ctx, id, b.Title, est); err != nil {
		return err
	}
	if cur, err := w.Store.BookByID(id, nil); err != nil || cur.Status != "processing" {
		return nil // wiped while waiting for token budget
	}

	w.setState("analyzing", id, b.Title)
	client := w.client()
	models := []string{w.Store.Setting(store.KeyLLMModel)}
	if fb := w.Store.Setting(store.KeyLLMFallbackModel); fb != "" {
		models = append(models, fb) // larger model only for edge-case failures
	}
	var lastErr error
	for _, model := range models {
		v, err := w.analyzeWith(ctx, client, model, id, user)
		if err == nil {
			return w.Store.SaveAnalysis(id, v.ToAnalysis(model))
		}
		lastErr = err
	}
	return lastErr
}

// client builds the configured provider: Claude through Anthropic's SDK, or
// any OpenAI-compatible API (OpenAI, Gemini, Perplexity, Ollama, vLLM...).
func (w *Worker) client() llm.Completer {
	return llm.New(w.Store.Setting(store.KeyLLMProvider), w.Store.Setting(store.KeyLLMBaseURL),
		w.Store.Setting(store.KeyLLMAPIKey), w.Store.SettingBool(store.KeyLLMJSONMode))
}

func (w *Worker) analyzeWith(ctx context.Context, c llm.Completer, model string, id int64, user string) (*llm.Verdict, error) {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	out, usage, err := c.Complete(cctx, model, llm.SystemPrompt, user)
	if usage.Total() > 0 {
		_ = w.Store.RecordUsage(id, model, usage.PromptTokens, usage.CompletionTokens)
	}
	if err != nil {
		return nil, err
	}
	return llm.ParseVerdict(out)
}

// waitForBudget blocks while the trailing-hour token total plus this call's
// estimate would exceed the configured cap (0 = unlimited).
func (w *Worker) waitForBudget(ctx context.Context, id int64, title string, est int) error {
	for {
		limit := w.Store.SettingInt(store.KeyTokensPerHour)
		if limit <= 0 {
			return nil
		}
		used, err := w.Store.TokensSince("-1 hour")
		if err != nil {
			return err
		}
		if used+est <= limit || used == 0 {
			return nil
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
