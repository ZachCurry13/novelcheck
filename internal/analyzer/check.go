package analyzer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// ErrCantSee means none of the configured AIs could read the photo (the
// model may not handle images, or the photo wasn't clear).
var ErrCantSee = errors.New("the AI couldn't read the photo")

// ReadCover asks the configured AIs, in order, which book a photo shows.
// Models that can't see images just fail, and the next one is tried.
func (w *Worker) ReadCover(ctx context.Context, image []byte, mediaType string) (enrich.Found, error) {
	var errs []string
	for _, ai := range w.Store.AIConfigs() {
		reader, ok := llm.New(ai.Provider, ai.BaseURL, ai.APIKey, ai.JSONMode).(llm.ImageReader)
		if !ok {
			continue
		}
		for _, model := range ai.Models {
			cctx, cancel := context.WithTimeout(ctx, llm.Timeout(ai.BaseURL, w.Store.SettingInt(store.KeyLLMTimeoutSeconds)))
			out, usage, err := reader.ReadImage(cctx, model, llm.CoverPrompt, image, mediaType)
			cancel()
			if usage.Total() > 0 {
				_ = w.Store.RecordUsageCost(0, model, usage.PromptTokens, usage.CompletionTokens, ai)
			}
			if err == nil {
				if f, ok := parseCover(out); ok {
					return f, nil
				}
				err = errors.New("no book title in the answer")
			}
			errs = append(errs, fmt.Sprintf("%s (%s): %v", ai.Name, model, err))
			if ctx.Err() != nil {
				return enrich.Found{}, ctx.Err()
			}
			if Unreachable(err) {
				break
			}
		}
	}
	if len(errs) == 0 {
		return enrich.Found{}, fmt.Errorf("%w: no AI is set up", ErrCantSee)
	}
	return enrich.Found{}, fmt.Errorf("%w (%s)", ErrCantSee, strings.Join(errs, "; "))
}

// parseCover reads {"title": "...", "author": "...", "isbn": "..."}.
func parseCover(out string) (enrich.Found, bool) {
	start, end := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return enrich.Found{}, false
	}
	var f enrich.Found
	if json.Unmarshal([]byte(out[start:end+1]), &f) != nil {
		return enrich.Found{}, false
	}
	f.Title, f.Author = strings.TrimSpace(f.Title), strings.TrimSpace(f.Author)
	f.ISBN = enrich.CleanISBN(f.ISBN)
	return f, f.Title != ""
}

// RateNow rates one book immediately for Check a book, alongside any batch.
// A parent standing in a shop is waiting, so it skips the queue, the scan
// delay and the hourly token limit. The book shows as "processing" meanwhile
// so a batch doesn't rate it twice; a failure is saved as a rating error.
func (w *Worker) RateNow(ctx context.Context, id int64) error {
	b, err := w.Store.BookByID(id, nil)
	if err != nil {
		return err
	}
	_ = w.Store.SetStatus(id, "processing", "")
	if b.Blurb == "" {
		w.fillBlurb(ctx, b)
	}
	a, err := w.askAIs(ctx, id, llm.UserPrompt(b.Title, b.Author, b.Blurb))
	if err != nil {
		_ = w.Store.SetStatus(id, "error", err.Error())
		return err
	}
	return w.Store.SaveAnalysis(id, *a)
}
