package deepread

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Note is what one part contained, shown in the book's window.
type Note struct {
	Label string `json:"label"`
	Level int    `json:"level"`
	Note  string `json:"note"`
}

// partTries is how often a part is tried before the scan fails; retryWait
// grows with each try (5 s, 20 s).
const partTries = 3

var retryWait = 5 * time.Second

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
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if r.Store.DeepReadStatus(d.ID) == "cancelled" {
			return nil
		}
		_ = r.Store.SetDeepProgress(d.ID, i, len(parts), model, prev)
		var res llm.PartResult
		// Up to 3 tries: a model can fumble its JSON or loop once. A part the
		// AI says is too long for it is split in two instead.
		for try := 1; ; try++ {
			res, model, err = ask(ctx, r, book.ID, system, llm.DeepPartUser(book.Title, book.Author, p.Label, i+1, len(parts), p.Text),
				func(out string) (llm.PartResult, error) { return llm.ParsePart(out) })
			if err == nil || ctx.Err() != nil || try == partTries {
				break
			}
			if llm.TooLong(err) {
				if a, b, ok := Halve(p); ok {
					parts = append(parts[:i], append([]Part{a, b}, parts[i+1:]...)...)
					p = a
					try-- // a split isn't a failed try; Halve stops at small parts
					continue
				}
			}
			if r.machineAway() {
				return errMachineAway // the Deep Scan machine went off: the scan waits for it
			}
			if !sleep(ctx, time.Duration(try*try)*retryWait) {
				return nil
			}
		}
		if ctx.Err() != nil {
			return nil // shutting down: it resumes after the restart
		}
		if err == nil {
			res, err = r.confirm(ctx, book, p, res) // a second look before a part can count as 3 or more
		}
		if ctx.Err() != nil {
			return nil
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
	raw, _ := json.Marshal(notes)
	if bigJump(prev, *a.SpiceLevel) {
		// A big raise waits for an admin, with the evidence, instead of applying.
		if err := r.Store.HoldDeepRead(d.ID, string(raw), a, *a.SpiceLevel); err != nil {
			return err
		}
		r.Store.Notify("warning", "deep-scan-review", fmt.Sprintf("Deep Scan suggests raising “%s” from Level %d to Level %d. Review it before it applies.",
			book.Title, *prev, *a.SpiceLevel), "#/deepscan?review")
		return nil
	}
	if err := r.Store.SaveAnalysis(book.ID, a); err != nil {
		return err
	}
	if err := r.Store.FinishDeepRead(d.ID, "done", string(raw), "", a.SpiceLevel); err != nil {
		return err
	}
	r.announce(book.Title, prev, *a.SpiceLevel)
	return nil
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
			start := time.Now()
			out, usage, err := client.Complete(cctx, model, system, user)
			cancel()
			if usage.Total() > 0 {
				_ = r.Store.RecordUsageCost(bookID, model, usage.PromptTokens, usage.CompletionTokens, ai, time.Since(start))
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
			if llm.HostDown(err) || llm.TooLong(err) {
				break // a down or stuck server (or a part too long for it): on to the backup AI
			}
		}
	}
	if len(errs) == 0 {
		return zero, "", errors.New("no AI is set up")
	}
	return zero, "", errors.New(strings.Join(errs, "; "))
}

// aiChain is the main AI (with the Deep Scan model, if set) then the backup.
func (r *Runner) aiChain() []store.AIConfig {
	if ai, ok := r.Store.DeepAI(); ok {
		return []store.AIConfig{ai} // its own machine: scans wait for it rather than using another
	}
	ais := r.Store.AIConfigs()
	if m := strings.TrimSpace(r.Store.Setting(store.KeyDeepModel)); m != "" && len(ais) > 0 {
		ais[0].Models = []string{m}
	}
	return ais
}

// sleep waits d, or reports false if the app is shutting down.
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
