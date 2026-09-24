// Package llm is a minimal OpenAI-compatible chat-completions client. It works
// with OpenAI, Gemini's OpenAI-compatible endpoint, Anthropic's compatibility
// layer, and local Ollama / vLLM servers.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL  string // e.g. https://api.openai.com/v1 or http://ollama:11434/v1
	APIKey   string
	JSONMode bool // send response_format=json_object (disable for servers that reject it)
	HTTP     *http.Client
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

func (u Usage) Total() int { return u.PromptTokens + u.CompletionTokens }

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string            `json:"model"`
	Messages       []message         `json:"messages"`
	Temperature    float64           `json:"temperature"`
	MaxTokens      int               `json:"max_tokens,omitempty"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete sends a system+user prompt and returns the assistant text and usage.
func (c *Client) Complete(ctx context.Context, model, system, user string) (string, Usage, error) {
	if strings.TrimSpace(c.BaseURL) == "" || strings.TrimSpace(model) == "" {
		return "", Usage{}, errors.New("LLM base URL and model must be configured")
	}
	body := chatRequest{
		Model:       model,
		Messages:    []message{{"system", system}, {"user", user}},
		Temperature: 0,
		MaxTokens:   600,
	}
	if c.JSONMode {
		body.ResponseFormat = map[string]string{"type": "json_object"}
	}
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(c.BaseURL, "/")+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return "", Usage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 120 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return "", Usage{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", Usage{}, err
	}
	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", Usage{}, fmt.Errorf("LLM %s: %s", resp.Status, truncate(string(raw), 200))
	}
	if cr.Error != nil {
		return "", cr.Usage, fmt.Errorf("LLM error: %s", cr.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", cr.Usage, fmt.Errorf("LLM %s", resp.Status)
	}
	if len(cr.Choices) == 0 {
		return "", cr.Usage, errors.New("LLM returned no choices")
	}
	return cr.Choices[0].Message.Content, cr.Usage, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
