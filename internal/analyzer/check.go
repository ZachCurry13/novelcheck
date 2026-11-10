package analyzer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// ErrCantSee means none of the configured AIs could read the photo (the
// model may not handle images, or the photo wasn't clear).
var ErrCantSee = errors.New("the AI couldn't read the photo")

// blind remembers the models that said they can't take a photo, so the next photo doesn't wait on them again
// (until the server restarts, in case the model is replaced by one that can).
var blind sync.Map

func cantSee(err error) bool {
	m := strings.ToLower(err.Error())
	for _, hint := range []string{"multimodal", "does not support image", "image input", "vision", "images are not supported"} {
		if strings.Contains(m, hint) {
			return true
		}
	}
	return false
}

// photoLimit is how long one model gets for a photo: someone is standing in a shop, so far less than the ten
// minutes a local model gets for rating a book.
const photoLimit = 75 * time.Second

// coverAttempts lists what to try for a photo, in order: the photo model chosen for the local AI (quick, free and
// never "busy"), then AIs on the internet, then the local AI's other models (which usually can't see).
func (w *Worker) coverAttempts() []store.AIConfig {
	ais := w.Store.AIConfigs()
	var out []store.AIConfig
	if vm := strings.TrimSpace(w.Store.Setting(store.KeyVisionModel)); vm != "" {
		for _, ai := range ais {
			if llm.IsLocal(ai.BaseURL) {
				first := ai
				first.Models = []string{vm}
				out = append(out, first)
				break
			}
		}
	}
	// No photo model chosen: a local model whose name says it can see (Gemma 3, LLaVA, …) still goes first.
	if len(out) == 0 {
		for _, ai := range ais {
			if !llm.IsLocal(ai.BaseURL) {
				continue
			}
			for _, m := range ai.Models {
				if looksVisual(m) {
					first := ai
					first.Models = []string{m}
					out = append(out, first)
				}
			}
		}
	}
	return append(out, visionOrder(ais)...)
}

// looksVisual says whether a local model's name is one of the families that take pictures.
func looksVisual(model string) bool {
	m := strings.ToLower(model)
	for _, fam := range []string{"gemma3", "llava", "vision", "moondream", "minicpm-v", "qwen2.5vl", "qwen2-vl", "bakllava", "pixtral"} {
		if strings.Contains(m, fam) {
			return true
		}
	}
	return false
}

// ReadCover asks the AIs which book a photo shows (see coverAttempts). Models that can't see images fail at
// once and are remembered, and the next one is tried.
func (w *Worker) ReadCover(ctx context.Context, image []byte, mediaType string) (enrich.Found, error) {
	var errs []string
	tried := map[string]bool{}
	for _, ai := range w.coverAttempts() {
		reader, ok := llm.New(ai.Provider, ai.BaseURL, ai.APIKey, ai.JSONMode).(llm.ImageReader)
		if !ok {
			continue
		}
		for _, model := range ai.Models {
			key := ai.BaseURL + "|" + model
			if tried[key] {
				continue
			}
			tried[key] = true
			if _, no := blind.Load(key); no {
				errs = append(errs, fmt.Sprintf("%s (%s): can't read photos", ai.Name, model))
				continue
			}
			limit := min(photoLimit, llm.Timeout(ai.BaseURL, w.Store.SettingInt(store.KeyLLMTimeoutSeconds)))
			cctx, cancel := context.WithTimeout(ctx, limit)
			start := time.Now()
			out, usage, err := reader.ReadImage(cctx, model, llm.CoverPrompt, image, mediaType)
			cancel()
			if usage.Total() > 0 {
				_ = w.Store.RecordUsageCost(0, model, usage.PromptTokens, usage.CompletionTokens, ai, time.Since(start))
			}
			if err == nil {
				if f, ok := parseCover(out); ok {
					return f, nil
				}
				err = errors.New("no book title in the answer")
			} else if cantSee(err) {
				blind.Store(key, true)
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
	hint := ""
	if strings.TrimSpace(w.Store.Setting(store.KeyVisionModel)) == "" {
		hint = " Tip: give your local AI a model that can read photos (Admin → AI → Photo-reading model, for example gemma3:4b)."
	}
	return enrich.Found{}, fmt.Errorf("%w (%s)%s", ErrCantSee, strings.Join(errs, "; "), hint)
}

// visionOrder puts AIs on the internet before AIs on the home network: a small
// local model is usually text-only and would only fail on a photo, so the
// cloud one that can see gets the first try. Otherwise the order is kept.
func visionOrder(ais []store.AIConfig) []store.AIConfig {
	out := make([]store.AIConfig, 0, len(ais))
	for _, ai := range ais {
		if !llm.IsLocal(ai.BaseURL) {
			out = append(out, ai)
		}
	}
	for _, ai := range ais {
		if llm.IsLocal(ai.BaseURL) {
			out = append(out, ai)
		}
	}
	return out
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
	return w.Store.SaveAnalysis(id, *unsureWithout(a, b.Blurb))
}

// RateWith rates one book again with the given AI (the bigger Deep Scan
// machine, for a rating the small model wasn't sure of). The old rating
// stays until the new one is saved; a failure leaves it as it was.
func (w *Worker) RateWith(ctx context.Context, id int64, ai store.AIConfig) error {
	b, err := w.Store.BookByID(id, nil)
	if err != nil {
		return err
	}
	if b.Blurb == "" {
		w.fillBlurb(ctx, b)
	}
	a, err := w.askWith(ctx, []store.AIConfig{ai}, id, llm.UserPrompt(b.Title, b.Author, b.Blurb))
	if err != nil {
		return err
	}
	return w.Store.SaveAnalysis(id, *unsureWithout(a, b.Blurb))
}
