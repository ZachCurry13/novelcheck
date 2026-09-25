package deepread

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Note is what one part contained, shown in the book's window.
type Note struct {
	Label string `json:"label"`
	Level int    `json:"level"`
	Note  string `json:"note"`
}

// scan reads one book part by part and saves the combined rating.
func (r *Runner) scan(ctx context.Context, d *store.DeepRead) error {
	book, err := r.Store.BookByID(d.BookID, nil)
	if err != nil {
		return nil // the book was removed meanwhile
	}
	parts, _, err := r.Prepare(d.BookID)
	if err != nil {
		return err
	}
	flags, _ := r.Store.CustomFlags()
	system := llm.DeepPartSystem(flags)
	prev := book.SpiceLevel
	var results []llm.PartResult
	model := ""
	for i, p := range parts {
		if r.Store.DeepReadStatus(d.ID) == "cancelled" {
			return nil
		}
		_ = r.Store.SetDeepProgress(d.ID, i, len(parts), model, prev)
		var res llm.PartResult
		res, model, err = ask(ctx, r, book.ID, system, llm.DeepPartUser(book.Title, book.Author, p.Label, i+1, len(parts), p.Text),
			func(out string) (llm.PartResult, error) { return llm.ParsePart(out) })
		if ctx.Err() != nil {
			return nil // shutting down: it resumes after the restart
		}
		if err != nil {
			return fmt.Errorf("part %d of %d (%s): %w", i+1, len(parts), p.Label, err)
		}
		results = append(results, res)
	}
	_ = r.Store.SetDeepProgress(d.ID, len(parts), len(parts), model, prev)

	a, notes := combine(parts, results)
	a.Model = store.DeepModelPrefix + model
	a.SpiceReason, a.SummaryVerdict = r.wrapUp(ctx, book, *a.SpiceLevel, notes)
	if a.SummaryVerdict == "" {
		a.SummaryVerdict = book.SummaryVerdict
	}
	if err := r.Store.SaveAnalysis(book.ID, a); err != nil {
		return err
	}
	raw, _ := json.Marshal(notes)
	if err := r.Store.FinishDeepRead(d.ID, "done", string(raw), "", a.SpiceLevel); err != nil {
		return err
	}
	r.announce(book.Title, prev, *a.SpiceLevel)
	return nil
}

// combine takes the highest level any part reached and every flag any part
// found. Notes keep the parts worth mentioning.
func combine(parts []Part, results []llm.PartResult) (store.Analysis, []Note) {
	level := 0
	var a store.Analysis
	var notes []Note
	for i, res := range results {
		level = max(level, res.Level)
		a.Nudity = a.Nudity || res.Nudity
		a.SoloActs = a.SoloActs || res.SoloActs
		a.HeavyInnuendo = a.HeavyInnuendo || res.HeavyInnuendo
		a.PlayfulFantasy = a.PlayfulFantasy || res.PlayfulFantasy
		a.DarkOccult = a.DarkOccult || res.DarkOccult
		a.DemonicPresence = a.DemonicPresence || res.DemonicPresence
		a.LGBTQContent = a.LGBTQContent || res.LGBTQ
		for _, k := range res.CustomFlags {
			if !slices.Contains(a.CustomFlags, k) {
				a.CustomFlags = append(a.CustomFlags, k)
			}
		}
		if res.Note != "" || res.Level >= 2 {
			notes = append(notes, Note{Label: parts[i].Label, Level: res.Level, Note: res.Note})
		}
	}
	a.SpiceLevel = &level
	return a, notes
}

// wrapUp asks for the short reason and the summary; if that fails, the
// note of the spiciest part serves as the reason.
func (r *Runner) wrapUp(ctx context.Context, book *store.Book, level int, notes []Note) (reason, summary string) {
	lines := make([]string, 0, len(notes))
	for _, n := range notes {
		lines = append(lines, fmt.Sprintf("%s (level %d): %s", n.Label, n.Level, n.Note))
	}
	type wrap struct{ reason, summary string }
	w, _, err := ask(ctx, r, book.ID, llm.DeepWrapUpSystem,
		llm.DeepWrapUpUser(book.Title, book.Author, level, lines, r.Store.Setting(store.KeyLanguage)),
		func(out string) (wrap, error) {
			rs, sm, ok := llm.ParseWrapUp(out)
			if !ok {
				return wrap{}, errors.New("no summary in the answer")
			}
			return wrap{rs, sm}, nil
		})
	if err == nil {
		return w.reason, w.summary
	}
	for _, n := range notes {
		if n.Level == level && n.Note != "" {
			return llm.ShortReason(n.Label + ": " + n.Note), ""
		}
	}
	return "", ""
}

// announce tells parents the result; a raised rating is a warning (it also
// goes to phones set to "Only problems").
func (r *Runner) announce(title string, prev *int, level int) {
	switch {
	case prev != nil && level > *prev:
		r.Store.Notify("warning", "deep-scan", fmt.Sprintf("Deep Scan raised “%s” from Level %d to Level %d.", title, *prev, level), "#/deepscan")
	case prev != nil && level < *prev:
		r.Store.NotifyRoutine("deep-scan-done", fmt.Sprintf("Deep Scan lowered “%s” from Level %d to Level %d.", title, *prev, level), "#/deepscan")
	default:
		r.Store.NotifyRoutine("deep-scan-done", fmt.Sprintf("Deep Scan finished: “%s” is Level %d.", title, level), "#/deepscan")
	}
}

// ask tries the AIs in order (the Deep Scan model first, if one is set)
// until one gives an answer parse accepts. It returns the model used.
func ask[T any](ctx context.Context, r *Runner, bookID int64, system, user string, parse func(string) (T, error)) (T, string, error) {
	var zero T
	var errs []string
	for _, ai := range r.aiChain() {
		client := llm.New(ai.Provider, ai.BaseURL, ai.APIKey, ai.JSONMode)
		for _, model := range ai.Models {
			cctx, cancel := context.WithTimeout(ctx, llm.Timeout(ai.BaseURL, r.Store.SettingInt(store.KeyLLMTimeoutSeconds)))
			out, usage, err := client.Complete(cctx, model, system, user)
			cancel()
			if usage.Total() > 0 {
				_ = r.Store.RecordUsageCost(bookID, model, usage.PromptTokens, usage.CompletionTokens, ai)
			}
			if err == nil {
				var v T
				if v, err = parse(out); err == nil {
					return v, model, nil
				}
			}
			if ctx.Err() != nil {
				return zero, "", ctx.Err()
			}
			errs = append(errs, fmt.Sprintf("%s AI (%s): %v", ai.Name, model, err))
		}
	}
	if len(errs) == 0 {
		return zero, "", errors.New("no AI is set up")
	}
	return zero, "", errors.New(strings.Join(errs, "; "))
}

// aiChain is the main AI (with the Deep Scan model, if set) then the backup.
func (r *Runner) aiChain() []store.AIConfig {
	ais := r.Store.AIConfigs()
	if m := strings.TrimSpace(r.Store.Setting(store.KeyDeepModel)); m != "" && len(ais) > 0 {
		ais[0].Models = []string{m}
	}
	return ais
}
