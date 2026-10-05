package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// toServer sends every request to srv, whatever host it was for.
type toServer struct{ srv *url.URL }

func (t toServer) RoundTrip(r *http.Request) (*http.Response, error) {
	r.URL.Scheme, r.URL.Host = t.srv.Scheme, t.srv.Host
	return http.DefaultTransport.RoundTrip(r)
}

// Gemini gets a low thinking setting and room to think, and its errors
// (sent as a list) come through as plain messages.
func TestGemini(t *testing.T) {
	var got map[string]any
	fail := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		if fail {
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `[{"error": {"code": 404, "message": "models/gemini-2.5-flash is not found for API version v1beta", "status": "NOT_FOUND"}}]`)
			return
		}
		io.WriteString(w, `{"choices": [{"message": {"role": "assistant", "content": "{\"ok\": true}"}}], "usage": {"prompt_tokens": 5, "completion_tokens": 3}}`)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	c := &Client{BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai", APIKey: "k", JSONMode: true,
		HTTP: &http.Client{Transport: toServer{u}}}

	out, _, err := c.Complete(context.Background(), "gemini-3.5-flash-lite", "system", "user")
	if err != nil || out != `{"ok": true}` {
		t.Fatalf("answer %q: %v", out, err)
	}
	if got["reasoning_effort"] != "low" || got["max_tokens"].(float64) < geminiMinTokens {
		t.Fatalf("request: %v", got)
	}

	fail = true
	_, _, err = c.Complete(context.Background(), "gemini-2.5-flash", "system", "user")
	if err == nil || !strings.Contains(err.Error(), "is not found for API version") || strings.Contains(err.Error(), `[{`) {
		t.Fatalf("error: %v", err)
	}

	// Other servers get neither.
	fail, got = false, nil
	c.BaseURL = "https://api.openai.com/v1"
	if _, _, err := c.Complete(context.Background(), "gpt-4o-mini", "system", "user"); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["reasoning_effort"]; ok || got["max_tokens"].(float64) != 600 {
		t.Fatalf("non-Gemini request: %v", got)
	}
}
