package analyzer

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/llm"
)

// Trace notes each AI asked while a person waits (which model, how long, and why it failed), for the bug log's
// Check a book section.
type Trace struct {
	mu    sync.Mutex
	steps []string
}

type traceKey struct{}

// WithTrace returns ctx carrying a new Trace.
func WithTrace(ctx context.Context) (context.Context, *Trace) {
	t := &Trace{}
	return context.WithValue(ctx, traceKey{}, t), t
}

func traceOf(ctx context.Context) *Trace { t, _ := ctx.Value(traceKey{}).(*Trace); return t }

func (t *Trace) add(ai, model string, took time.Duration, err error) {
	if t == nil {
		return
	}
	res := "ok"
	if d := llm.DailyLimit(err); d != nil {
		res = "daily limit reached, asked again in " + d.Wait()
	} else if err != nil {
		res = err.Error()
		if len(res) > 160 {
			res = res[:160] + "…"
		}
	}
	t.mu.Lock()
	t.steps = append(t.steps, fmt.Sprintf("%s %s %.1fs %s", ai, model, took.Seconds(), res))
	t.mu.Unlock()
}

// String is the steps, oldest first.
func (t *Trace) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.Join(t.steps, " · ")
}
