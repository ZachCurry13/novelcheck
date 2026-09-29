// Package aitools looks after the family's own AI machines: it checks once
// a day whether the Ollama models in use have a newer version (and says so,
// with an Update button in Admin), and times each model on a sample book
// (the speed test) so estimates can say how long a Deep Scan takes.
package aitools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/ollama"
	"github.com/zachcurry13/novelcheck/internal/safe"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// keyNotified remembers which updates were announced (model@server → digest).
const keyNotified = "model_updates_notified"

// Service holds the latest update check and speed test.
type Service struct {
	Store *store.Store

	mu        sync.Mutex
	updates   []ollama.Update
	checkedAt time.Time
	checkErr  string
	benching  bool
}

func New(st *store.Store) *Service { return &Service{Store: st} }

// machines are the AIs to look after: main, backup and the Deep Scan
// machine, each with the models it uses.
func (s *Service) machines() []store.AIConfig {
	ais := s.Store.AIConfigs()
	if m := strings.TrimSpace(s.Store.Setting(store.KeyDeepModel)); m != "" && len(ais) > 0 {
		ais[0].Models = append(ais[0].Models, m)
	}
	if ai, ok := s.Store.DeepAI(); ok {
		ais = append(ais, ai)
	}
	return ais
}

// Loop checks for model updates once a day (the first time 15 minutes
// after start).
func (s *Service) Loop(ctx context.Context) {
	wait := 15 * time.Minute
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = 24 * time.Hour
		_ = safe.Run("model update check", func() error { s.CheckUpdates(ctx); return nil })
	}
}

// CheckUpdates asks each local Ollama and the model library, and announces
// new versions once each.
func (s *Service) CheckUpdates(ctx context.Context) []ollama.Update {
	var all []ollama.Update
	var errs []string
	for _, ai := range s.machines() {
		if ai.Provider == "anthropic" || !llm.IsLocal(ai.BaseURL) {
			continue
		}
		base, err := ollama.Normalize(ai.BaseURL)
		if err != nil {
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		ups, err := ollama.Outdated(cctx, base, ai.Models)
		cancel()
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s AI: %v", ai.Name, err))
			continue
		}
		all = append(all, ups...)
	}
	s.mu.Lock()
	s.updates, s.checkedAt, s.checkErr = all, time.Now(), strings.Join(errs, "; ")
	s.mu.Unlock()
	s.announce(all)
	return all
}

func (s *Service) announce(ups []ollama.Update) {
	seen := map[string]string{}
	_ = json.Unmarshal([]byte(s.Store.Setting(keyNotified)), &seen)
	for _, u := range ups {
		key := u.Model + "@" + u.Server
		if seen[key] == u.Digest {
			continue
		}
		seen[key] = u.Digest
		s.Store.Notify("info", "model-update", fmt.Sprintf("💡 A newer version of %s is out. Update it under Admin → AI & Scans → AI machines.", u.Model),
			"#/admin?tab=ai")
	}
	if data, err := json.Marshal(seen); err == nil {
		_ = s.Store.SetSetting(keyNotified, string(data))
	}
}

// Status is what Admin shows.
type Status struct {
	Updates   []ollama.Update        `json:"updates"`
	CheckedAt string                 `json:"checked_at"`
	CheckErr  string                 `json:"check_error"`
	Benching  bool                   `json:"benching"`
	Results   map[string]store.Bench `json:"results"`
}

// Status returns the latest check and speed test.
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := Status{Updates: s.updates, CheckErr: s.checkErr, Benching: s.benching, Results: s.Store.Benchmarks()}
	if st.Updates == nil {
		st.Updates = []ollama.Update{}
	}
	if !s.checkedAt.IsZero() {
		st.CheckedAt = s.checkedAt.UTC().Format(time.RFC3339)
	}
	return st
}

// Updated forgets an update once its model was pulled again.
func (s *Service) Updated(server, model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.updates[:0]
	for _, u := range s.updates {
		if !(u.Model == model && u.Server == server) {
			kept = append(kept, u)
		}
	}
	s.updates = kept
	log.Printf("model %s updated on %s", model, server)
}
