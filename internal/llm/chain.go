package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Ask sends one question to each AI in ais (the main one, then the backup),
// each model in order, until parse accepts an answer. It returns the model
// that answered. Token use is recorded against bookID (0 for none).
func Ask(ctx context.Context, st *store.Store, ais []store.AIConfig, bookID int64, system, user string, parse func(string) error) (string, error) {
	var errs []string
	for _, ai := range ais {
		client := New(ai.Provider, ai.BaseURL, ai.APIKey, ai.JSONMode)
		for _, model := range ai.Models {
			cctx, cancel := context.WithTimeout(ctx, Timeout(ai.BaseURL, st.SettingInt(store.KeyLLMTimeoutSeconds)))
			out, usage, err := client.Complete(cctx, model, system, user)
			cancel()
			if usage.Total() > 0 {
				_ = st.RecordUsageCost(bookID, model, usage.PromptTokens, usage.CompletionTokens, ai)
			}
			if err == nil {
				if err = parse(out); err == nil {
					return model, nil
				}
			}
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			errs = append(errs, fmt.Sprintf("%s AI (%s): %v", ai.Name, model, err))
		}
	}
	if len(errs) == 0 {
		return "", errors.New("no AI is set up")
	}
	return "", errors.New(strings.Join(errs, "; "))
}
