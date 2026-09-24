package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/llm"
)

// fakeAnthropic mimics POST /v1/messages.
func fakeAnthropic(t *testing.T, status int, body map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("X-Api-Key") != "sk-test" {
			http.Error(w, `{"type":"error","error":{"type":"authentication_error","message":"bad key"}}`, 401)
			return
		}
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["model"] != "claude-haiku-4-5" || req["system"] == nil {
			t.Errorf("unexpected request %v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
}

func TestAnthropicClient(t *testing.T) {
	srv := fakeAnthropic(t, 200, map[string]any{
		"id": "msg_1", "type": "message", "role": "assistant", "model": "claude-haiku-4-5",
		"content":     []any{map[string]any{"type": "text", "text": sample}},
		"stop_reason": "end_turn",
		"usage":       map[string]int{"input_tokens": 700, "output_tokens": 90},
	})
	defer srv.Close()
	c := &llm.AnthropicClient{APIKey: "sk-test", BaseURL: srv.URL}
	out, usage, err := c.Complete(context.Background(), "claude-haiku-4-5", llm.SystemPrompt, llm.UserPrompt("T", "A", "B"))
	if err != nil {
		t.Fatal(err)
	}
	if usage.Total() != 790 {
		t.Fatalf("usage %+v", usage)
	}
	if v, err := llm.ParseVerdict(out); err != nil || v.Classification != "Closed Door" {
		t.Fatalf("verdict %v %v", v, err)
	}
}

func TestAnthropicClientErrors(t *testing.T) {
	srv := fakeAnthropic(t, 200, nil)
	defer srv.Close()
	c := &llm.AnthropicClient{APIKey: "wrong", BaseURL: srv.URL}
	if _, _, err := c.Complete(context.Background(), "claude-haiku-4-5", "s", "u"); err == nil || !strings.Contains(err.Error(), "API key") {
		t.Fatalf("expected API key error, got %v", err)
	}
	if _, _, err := (&llm.AnthropicClient{}).Complete(context.Background(), "claude-haiku-4-5", "s", "u"); err == nil {
		t.Fatal("expected missing-key error")
	}

	refusal := fakeAnthropic(t, 200, map[string]any{
		"id": "msg_2", "type": "message", "role": "assistant", "model": "claude-haiku-4-5",
		"content": []any{}, "stop_reason": "refusal",
		"usage": map[string]int{"input_tokens": 10, "output_tokens": 0},
	})
	defer refusal.Close()
	c = &llm.AnthropicClient{APIKey: "sk-test", BaseURL: refusal.URL}
	if _, _, err := c.Complete(context.Background(), "claude-haiku-4-5", "s", "u"); err == nil || !strings.Contains(err.Error(), "declined") {
		t.Fatalf("expected refusal error, got %v", err)
	}
}
