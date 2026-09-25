package suggest

// Service puts one reader's suggestions together: the AI's latest picks
// (made at most once a day, see ai.go) that are still available, topped up
// from the free matching, plus books the family doesn't own when the admin
// allows them.

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

const (
	show         = 10 // suggestions shown
	shortlist    = 30 // matches the AI chooses from
	refreshAfter = 24 * time.Hour
	retryAfter   = time.Hour // after the AI failed, or most of its picks are gone
)

type Service struct {
	Store   *store.Store
	mu      sync.Mutex
	running map[int64]bool // readers whose AI picks are being made now
}

func New(st *store.Store) *Service { return &Service{Store: st, running: map[int64]bool{}} }

// Item is one suggested library book.
type Item struct {
	Book   *store.Book `json:"book"`
	Reason string      `json:"reason"`
	ByAI   bool        `json:"by_ai"`
}

// Result is what the Suggested Reads section shows.
type Result struct {
	Items      []Item               `json:"items"`
	Outside    []llm.SuggestOutside `json:"outside"`           // books the family doesn't own
	Mode       string               `json:"mode"`              // free, ai or ai_outside
	Refreshing bool                 `json:"refreshing"`        // the AI is picking new ones now
	MadeAt     string               `json:"made_at,omitempty"` // when the AI last picked
	Up         int                  `json:"up"`                // the reader's 👍 and 👎 so far
	Down       int                  `json:"down"`
}

// saved is the AI's answer as stored in suggestion_sets.
type saved struct {
	Picks   []llm.SuggestPick    `json:"picks"`
	Outside []llm.SuggestOutside `json:"outside"`
	Error   string               `json:"error,omitempty"`
}

// For returns u's suggestions, starting a new AI pick in the background when due.
func (s *Service) For(u *store.User) (Result, error) {
	mode := s.Store.Setting(store.KeySuggestMode)
	pool, err := s.Store.SuggestPool(u)
	if err != nil {
		return Result{}, err
	}
	signals, err := s.Store.SuggestSignals(u.ID)
	if err != nil {
		return Result{}, err
	}
	picks := Rank(pool, signals, s.Store.FamilyReads(u.ID), shortlist)
	res := Result{Items: []Item{}, Outside: []llm.SuggestOutside{}, Mode: modeName(mode)}
	liked := false
	for _, sg := range signals {
		switch sg.Status {
		case "up", "want", "liked":
			res.Up++
		case "down", "read", "notwant", "disliked":
			res.Down++
		}
		liked = liked || sg.Weight > 0
	}
	available := make(map[int64]bool, len(pool))
	for _, b := range pool {
		available[b.ID] = true
	}

	var set saved
	if mode != store.SuggestFree {
		cur := s.Store.SuggestionSetFor(u.ID)
		valid := 0
		if cur != nil && cur.Mode == mode && json.Unmarshal([]byte(cur.Data), &set) == nil {
			res.MadeAt = cur.MadeAt.Format(time.RFC3339)
			for _, p := range set.Picks {
				if available[p.ID] {
					valid++
				}
			}
		} else {
			set = saved{}
		}
		if liked && len(picks) > 0 && due(cur, mode, set, valid) {
			res.Refreshing = s.refresh(*u, mode, picks, signals)
		}
	}

	seen := map[int64]bool{}
	add := func(id int64, reason string, byAI bool) {
		if seen[id] || !available[id] || len(res.Items) >= show {
			return
		}
		if b, err := s.Store.BookByID(id, u); err == nil {
			seen[id] = true
			res.Items = append(res.Items, Item{b, reason, byAI})
		}
	}
	for _, p := range set.Picks {
		add(p.ID, p.Reason, true)
	}
	for _, p := range picks {
		add(p.Book.ID, p.Reason, false)
	}
	if mode == store.SuggestAIOutside && outsideAllowed(u) {
		voted := s.Store.Voted(u.ID)
		for _, o := range set.Outside {
			if voted[store.NormKey(o.Title, o.Author)] == 0 && s.Store.MatchBook(o.Title, o.Author) == 0 {
				res.Outside = append(res.Outside, o)
			}
		}
	}
	return res, nil
}

// due reports whether the AI should pick again: never picked, the setting
// changed, a day has passed, or (after an hour) it failed or most of its
// picks have been queued or voted on.
func due(cur *store.SuggestionSet, mode string, set saved, valid int) bool {
	if cur == nil || cur.Mode != mode {
		return true
	}
	age := time.Since(cur.MadeAt)
	return age > refreshAfter || (age > retryAfter && (set.Error != "" || valid < min(3, len(set.Picks))))
}

// outsideAllowed: books the family doesn't own aren't rated yet, so kids'
// accounts and anyone hiding unrated books never get them.
func outsideAllowed(u *store.User) bool { return u.Role != store.RoleRestricted && !u.HideUnrated }

func modeName(mode string) string {
	switch mode {
	case store.SuggestFree:
		return "free"
	case store.SuggestAI:
		return "ai"
	}
	return "ai_outside"
}
