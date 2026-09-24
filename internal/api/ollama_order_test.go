package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Picking several Ollama models saves them in order: #1 main, the rest fallbacks.
func TestOllamaUseModelOrder(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	ol := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			fmt.Fprint(w, `{"version":"0.12.0"}`)
		case "/api/tags":
			fmt.Fprint(w, `{"models":[{"name":"llama3.2:latest"},{"name":"qwen2.5:7b"},{"name":"llama3.1:8b"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ol.Close()

	res, out := admin.do("POST", "/api/admin/ollama/use", map[string]any{
		"url": ol.URL, "models": []string{"qwen2.5:7b", "llama3.1:8b", "llama3.2:latest"}}, true)
	if res.StatusCode != 200 || out["model"] != "qwen2.5:7b" || out["fallback"] != "llama3.1:8b,llama3.2:latest" {
		t.Fatalf("use: %d %v", res.StatusCode, out)
	}
	if got := st.LLMModels(); len(got) != 3 || got[2] != "llama3.2:latest" {
		t.Fatalf("saved order: %v", got)
	}
	if st.Setting(store.KeyLLMBaseURL) != ol.URL+"/v1" {
		t.Fatalf("base url: %s", st.Setting(store.KeyLLMBaseURL))
	}
	if res, _ := admin.do("POST", "/api/admin/ollama/use", map[string]any{"url": ol.URL, "models": []string{"qwen2.5:7b", "nope:1b"}}, true); res.StatusCode != 400 {
		t.Fatalf("unknown model must be refused: %d", res.StatusCode)
	}
	// The single-model form still works and clears old fallbacks.
	if res, out := admin.do("POST", "/api/admin/ollama/use", map[string]any{"url": ol.URL, "model": "llama3.2"}, true); res.StatusCode != 200 || out["fallback"] != "" {
		t.Fatalf("single model: %d %v", res.StatusCode, out)
	}
}
