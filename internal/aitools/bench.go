package aitools

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/safe"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The speed test's made-up book: a rating from its blurb, and one Deep Scan
// part of about 600 words.
const (
	sampleTitle  = "The Lantern Keeper"
	sampleAuthor = "A. Sample"
	sampleBlurb  = "When the lighthouse on Gull Island goes dark, twelve-year-old Mara must keep the lantern burning through a winter of storms, while a quiet new boy in the village helps her uncover why the old keeper left."
	sampleWords  = "Mara climbed the spiral stairs with the oil can swinging from her hand. The wind pushed against the tower and the glass hummed like a held note. Below, the sea broke white over the rocks, and the fishing boats rocked in the harbor with their lamps already lit. She trimmed the wick, wiped the lens with her sleeve, and waited for the flame to steady. Tomas knocked at the door at the bottom of the stairs, carrying bread and a letter he had found in the old keeper's desk. They read it by the lantern, sitting on the cold floor, while the beam swept slowly over the dark water. "
)

// ErrBusy means a speed test is already running.
var ErrBusy = errors.New("a speed test is already running")

// sampleText is about 600 words of the sample book.
func sampleText() string {
	var b strings.Builder
	for b.Len() < 3400 {
		b.WriteString(sampleWords)
	}
	return b.String()
}

// StartBench times every model of one machine ("main", "backup" or "deep")
// in the background.
func (s *Service) StartBench(which string) error {
	var ai store.AIConfig
	found := false
	switch which {
	case "deep":
		if ai, found = s.Store.DeepAI(); !found {
			if ais := s.Store.AIConfigs(); len(ais) > 0 { // Deep Scans on the main machine
				ai, found = ais[0], true
				if m := strings.TrimSpace(s.Store.Setting(store.KeyDeepModel)); m != "" {
					ai.Models = []string{m}
				}
			}
		}
	default:
		for _, a := range s.Store.AIConfigs() {
			if a.Name == which {
				ai, found = a, true
			}
		}
	}
	if !found || len(ai.Models) == 0 {
		return errors.New("that AI isn't set up")
	}
	s.mu.Lock()
	if s.benching {
		s.mu.Unlock()
		return ErrBusy
	}
	s.benching = true
	s.mu.Unlock()
	safe.Go("speed test", func() {
		defer func() {
			s.mu.Lock()
			s.benching = false
			s.mu.Unlock()
		}()
		s.bench(ai)
	})
	return nil
}

func (s *Service) bench(ai store.AIConfig) {
	client := llm.New(ai.Provider, ai.BaseURL, ai.APIKey, ai.JSONMode)
	flags, _ := s.Store.CustomFlags()
	text := sampleText()
	for _, model := range ai.Models {
		b := store.Bench{AI: ai.Name, Model: model, At: time.Now().UTC().Format(time.RFC3339), PartWords: len(strings.Fields(text))}
		call := func(system, user string) (time.Duration, llm.Usage, error) {
			ctx, cancel := context.WithTimeout(context.Background(), llm.Timeout(ai.BaseURL, s.Store.SettingInt(store.KeyLLMTimeoutSeconds)))
			defer cancel()
			start := time.Now()
			_, u, err := client.Complete(ctx, model, system, user)
			took := time.Since(start)
			if u.Total() > 0 {
				_ = s.Store.RecordUsageCost(0, model, u.PromptTokens, u.CompletionTokens, ai, took)
			}
			return took, u, err
		}
		took, _, err := call(llm.SystemPromptFor(s.Store.Setting(store.KeyLanguage), flags), llm.UserPrompt(sampleTitle, sampleAuthor, sampleBlurb))
		b.RatingSeconds = took.Seconds()
		if err == nil {
			var u llm.Usage
			took, u, err = call(llm.DeepPartSystem(flags), llm.DeepPartUser(sampleTitle, sampleAuthor, "Chapter 1", 1, 1, text))
			b.PartSeconds = took.Seconds()
			if secs := took.Seconds(); secs > 0 {
				b.TokensPerSecond = float64(u.CompletionTokens) / secs
			}
		}
		if err != nil {
			b.Error = err.Error()
		}
		_ = s.Store.SaveBench(b)
	}
}
