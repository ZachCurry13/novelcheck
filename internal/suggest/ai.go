package suggest

// The AI's part: from the free matching's shortlist it picks the best books
// for the reader with a reason each, and (if allowed) names books the
// family doesn't own. It runs in the background, at most once a day per
// reader, and stays within the hourly token cap.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/safe"
	"github.com/zachcurry13/novelcheck/internal/store"
)

const estTokens = 4000 // a typical suggestion call, for the hourly cap

// refresh starts the AI picking for u unless it already is, or no AI is set
// up, or this hour's token budget is spent. It reports whether picks are
// being made.
func (s *Service) refresh(u store.User, mode string, picks []Pick, signals []store.SuggestSignal) bool {
	if !s.hasAI() || !s.withinBudget() {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running[u.ID] {
		return true
	}
	s.running[u.ID] = true
	safe.Go("suggested reads", func() {
		defer func() {
			s.mu.Lock()
			delete(s.running, u.ID)
			s.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		res, err := s.ask(ctx, &u, mode, picks, signals)
		set := saved{Picks: res.Picks, Outside: res.Outside}
		if err != nil {
			log.Printf("suggested reads for %s: %v", u.Username, err)
			set.Error = err.Error()
		}
		data, _ := json.Marshal(set)
		_ = s.Store.SaveSuggestionSet(u.ID, mode, string(data))
	})
	return true
}

// ask tries the main AI then the backup, each model in order.
func (s *Service) ask(ctx context.Context, u *store.User, mode string, picks []Pick, signals []store.SuggestSignal) (llm.SuggestResult, error) {
	outside := 0
	if mode == store.SuggestAIOutside && outsideAllowed(u) {
		outside = 5
	}
	system := llm.SuggestSystem(outside, s.Store.Setting(store.KeyLanguage))
	user := llm.SuggestUser(reader(u, signals), candidates(picks))
	ids := make(map[int64]bool, len(picks))
	for _, p := range picks {
		ids[p.Book.ID] = true
	}
	var errs []string
	for _, ai := range s.Store.AIConfigs() {
		client := llm.New(ai.Provider, ai.BaseURL, ai.APIKey, ai.JSONMode)
		for _, model := range ai.Models {
			cctx, cancel := context.WithTimeout(ctx, llm.Timeout(ai.BaseURL, s.Store.SettingInt(store.KeyLLMTimeoutSeconds)))
			out, usage, err := client.Complete(cctx, model, system, user)
			cancel()
			if usage.Total() > 0 {
				_ = s.Store.RecordUsageCost(0, model, usage.PromptTokens, usage.CompletionTokens, ai)
			}
			if err == nil {
				var res llm.SuggestResult
				if res, err = llm.ParseSuggest(out, ids); err == nil {
					return res, nil
				}
			}
			if ctx.Err() != nil {
				return llm.SuggestResult{}, ctx.Err()
			}
			errs = append(errs, fmt.Sprintf("%s AI (%s): %v", ai.Name, model, err))
		}
	}
	return llm.SuggestResult{}, fmt.Errorf("%s", strings.Join(errs, "; "))
}

func (s *Service) hasAI() bool {
	for _, ai := range s.Store.AIConfigs() {
		if len(ai.Models) > 0 {
			return true
		}
	}
	return false
}

// withinBudget: suggestions never push past the hourly token cap (0 = none).
func (s *Service) withinBudget() bool {
	limit := s.Store.SettingInt(store.KeyTokensPerHour)
	used, err := s.Store.TokensSince("-1 hour")
	return limit <= 0 || (err == nil && used+estTokens <= limit)
}

var statusWords = map[string]string{"queued": "wants to read", "reading": "reading now",
	"finished": "finished", "up": "liked this suggestion", "read": "has already read it", "down": "doesn't want",
	"want": "wants to read", "liked": "read it and liked it", "notwant": "doesn't want to read it", "disliked": "read it and didn't like it"}

// reasonWords explain a 👎 to the AI.
var reasonWords = map[string]string{"story": "not their kind of story", "author": "doesn't like the author",
	"series": "not interested in the series", "spicy": "too spicy for them"}

// reader describes u for the AI: up to 25 liked books (reading and finished
// first), 10 unwanted ones, and their content limits.
func reader(u *store.User, signals []store.SuggestSignal) llm.SuggestReader {
	var r llm.SuggestReader
	for _, pass := range [][]string{{"reading", "finished", "liked", "up", "read"}, {"queued", "want"}} {
		for _, sg := range signals {
			if slices.Contains(pass, sg.Status) && len(r.Liked) < 25 {
				r.Liked = append(r.Liked, describe(sg))
			}
		}
	}
	for _, sg := range signals {
		if sg.Weight < 0 && len(r.Disliked) < 10 {
			r.Disliked = append(r.Disliked, describe(sg))
		}
	}
	r.Limits = limits(u)
	return r
}

func describe(sg store.SuggestSignal) string {
	d := sg.Title
	if sg.Author != "" {
		d += " by " + sg.Author
	}
	why := statusWords[sg.Status]
	if w := reasonWords[sg.Reason]; w != "" {
		why += ": " + w
	}
	return d + " (" + why + ")"
}

// limits is u's content rules in plain words.
func limits(u *store.User) string {
	var l []string
	if u.Role == store.RoleRestricted {
		l = append(l, "this reader is a child")
	}
	for _, g := range store.AgeGroups {
		if u.AgeLevel == g.Level {
			l = append(l, "age group "+g.Label)
		}
	}
	if u.MaxSpice >= 0 {
		l = append(l, fmt.Sprintf("pepper level %d of 5 at most (0 = no romance, 5 = explicit)", u.MaxSpice))
	}
	for _, r := range []struct {
		on   bool
		text string
	}{{u.HideOpenDoor, "no open-door sex scenes"}, {u.HideNudity, "no nudity"}, {u.HideSoloActs, "no solo sexual acts"},
		{u.HideInnuendo, "no heavy innuendo"}, {u.HideDarkOccult, "no dark occult or demonic themes"}, {u.HideLGBTQ, "no LGBTQ+ content"}} {
		if r.on {
			l = append(l, r.text)
		}
	}
	return strings.Join(l, "; ")
}

func candidates(picks []Pick) []llm.SuggestCandidate {
	out := make([]llm.SuggestCandidate, len(picks))
	for i, p := range picks {
		b := p.Book
		out[i] = llm.SuggestCandidate{ID: b.ID, Title: b.Title, Author: b.Author, Series: seriesLabel(b), Description: b.Description, Level: b.Spice}
	}
	return out
}

func seriesLabel(b store.SuggestBook) string {
	if b.Series == "" {
		return ""
	}
	if b.SeriesIndex > 0 {
		return fmt.Sprintf("%s #%g", b.Series, b.SeriesIndex)
	}
	return b.Series
}
