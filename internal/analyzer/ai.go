package analyzer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// askAIs rates a book with the main AI's models in order, then, if they all
// fail, the backup AI's. When a server can't be reached at all its other
// models are skipped, so a switched-off Ollama doesn't cost a wait per model.
func (w *Worker) askAIs(ctx context.Context, id int64, user string) (*store.Analysis, error) {
	var errs []string
	var lastErr error
	for _, ai := range w.Store.AIConfigs() {
		if len(ai.Models) == 0 {
			lastErr = errors.New("LLM base URL and model must be configured")
			errs = append(errs, ai.Name+" AI: no model set")
			continue
		}
		client := llm.New(ai.Provider, ai.BaseURL, ai.APIKey, ai.JSONMode)
		for _, model := range ai.Models {
			v, err := w.analyzeWith(ctx, ai, client, model, id, user)
			if err == nil {
				if ai.Name == "backup" {
					w.Store.Notify("warning", "llm-backup",
						"The main AI isn't working, so the backup AI rated some books. Check System → Check everything.", "#/system")
				} else {
					w.Store.Resolve("llm-backup")
				}
				a := v.ToAnalysis(model)
				return &a, nil
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = err
			errs = append(errs, fmt.Sprintf("%s AI (%s): %v", ai.Name, model, err))
			if Unreachable(err) {
				break // the server is down: its other models won't answer either
			}
		}
	}
	if len(errs) > 1 {
		return nil, errors.New(joinErrs(errs))
	}
	return nil, lastErr
}

func joinErrs(errs []string) string {
	out := errs[0]
	for _, e := range errs[1:] {
		out += " · then " + e
	}
	return out
}

// Unreachable reports whether err means the AI server couldn't be contacted
// at all (switched off, wrong address), as opposed to a slow or bad answer.
func Unreachable(err error) bool {
	var op *net.OpError
	if errors.As(err, &op) && op.Op == "dial" {
		return true
	}
	var dns *net.DNSError
	return errors.As(err, &dns)
}

func (w *Worker) analyzeWith(ctx context.Context, ai store.AIConfig, c llm.Completer, model string, id int64, user string) (*llm.Verdict, error) {
	limit := llm.Timeout(ai.BaseURL, w.Store.SettingInt(store.KeyLLMTimeoutSeconds))
	cctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	out, usage, err := c.Complete(cctx, model, llm.SystemPromptFor(w.Store.Setting(store.KeyLanguage)), user)
	if usage.Total() > 0 {
		_ = w.Store.RecordUsageCost(id, model, usage.PromptTokens, usage.CompletionTokens, ai)
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return nil, timeoutError(model, limit, llm.IsLocal(ai.BaseURL) && ai.Provider != "anthropic")
		}
		return nil, err
	}
	return llm.ParseVerdict(out)
}

// timeoutError explains a model that didn't answer in time.
func timeoutError(model string, limit time.Duration, local bool) error {
	msg := fmt.Sprintf("%s didn't answer within %s", model, limit.Round(time.Second))
	if local {
		msg += ". On your own server this usually means the model is running on the CPU instead of the GPU " +
			"(see Usage → Ollama: it should say 100% GPU), or it's too big for your GPU. Try a smaller model, " +
			"or raise the AI time limit in Admin → LLM Analysis Engine."
	} else {
		msg += ". The AI service may be overloaded; it will be retried next batch. You can raise the AI time limit in Admin."
	}
	return errors.New(msg)
}
