package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Completer is implemented by every provider client the analyzer can use.
type Completer interface {
	Complete(ctx context.Context, model, system, user string) (string, Usage, error)
}

// AnthropicClient calls Claude through Anthropic's official Go SDK (the
// native Messages API). BaseURL is only overridden in tests.
type AnthropicClient struct {
	APIKey  string
	BaseURL string
}

// Complete sends one system+user request and returns Claude's text.
func (c *AnthropicClient) Complete(ctx context.Context, model, system, user string) (string, Usage, error) {
	if strings.TrimSpace(c.APIKey) == "" || strings.TrimSpace(model) == "" {
		return "", Usage{}, errors.New("Claude needs an Anthropic API key and a model")
	}
	opts := []option.RequestOption{option.WithAPIKey(c.APIKey)}
	if c.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(c.BaseURL))
	}
	client := anthropic.NewClient(opts...)
	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 1024, // the JSON verdict is ~200 tokens
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
		},
	})
	if err != nil {
		var apierr *anthropic.Error
		if errors.As(err, &apierr) {
			switch apierr.StatusCode {
			case 401, 403:
				return "", Usage{}, errors.New("Claude rejected the API key")
			case 404:
				return "", Usage{}, fmt.Errorf("Claude model %q not found", model)
			case 429:
				return "", Usage{}, errors.New("Claude rate limit reached; try a smaller batch or a longer scan delay")
			}
			return "", Usage{}, fmt.Errorf("Claude API error %d", apierr.StatusCode)
		}
		return "", Usage{}, err
	}
	usage := Usage{PromptTokens: int(resp.Usage.InputTokens), CompletionTokens: int(resp.Usage.OutputTokens)}
	if resp.StopReason == anthropic.StopReasonRefusal {
		return "", usage, errors.New("Claude declined to rate this book")
	}
	var text strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	if text.Len() == 0 {
		return "", usage, errors.New("Claude returned no text")
	}
	return text.String(), usage, nil
}

// New returns the client for a provider: "anthropic" uses Claude's native
// API; anything else is treated as an OpenAI-compatible endpoint.
func New(provider, baseURL, apiKey string, jsonMode bool) Completer {
	if provider == "anthropic" {
		return &AnthropicClient{APIKey: apiKey}
	}
	return &Client{BaseURL: baseURL, APIKey: apiKey, JSONMode: jsonMode}
}
