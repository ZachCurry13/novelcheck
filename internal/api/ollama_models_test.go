package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestOllamaModelCleanup(t *testing.T) {
	var deleted []string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/tags":
			_, _ = w.Write([]byte(`{"models": [
				{"name": "llama3.2:3b", "size": 2019393189, "details": {"parameter_size": "3.2B", "quantization_level": "Q4_K_M"}},
				{"name": "qwen2.5:14b", "size": 8988124069, "details": {"parameter_size": "14.8B", "quantization_level": "Q4_K_M"}}]}`))
		case r.URL.Path == "/api/delete" && r.Method == http.MethodDelete:
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			deleted = append(deleted, body["model"])
		default:
			http.NotFound(w, r)
		}
	}))
	defer fake.Close()
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	_ = st.SetSetting(store.KeyLLMModel, "llama3.2:3b")

	_, out := admin.do("GET", "/api/admin/ollama/models?url="+fake.URL, nil, false)
	models := out["models"].([]any)
	first := models[0].(map[string]any)
	if len(models) != 2 || first["name"] != "qwen2.5:14b" || first["in_use"] != false || out["total"].(float64) != 2019393189+8988124069 {
		t.Fatalf("models (largest first, with totals): %v", out)
	}
	if models[1].(map[string]any)["in_use"] != true {
		t.Fatal("the model NovelCheck uses is marked")
	}
	if res, _ := admin.do("POST", "/api/admin/ollama/delete", map[string]string{"url": fake.URL, "model": "llama3.2:3b"}, true); res.StatusCode != 409 {
		t.Fatalf("deleting the model in use: %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", "/api/admin/ollama/delete", map[string]string{"url": fake.URL, "model": "qwen2.5:14b"}, true); res.StatusCode != 200 || len(deleted) != 1 || deleted[0] != "qwen2.5:14b" {
		t.Fatalf("delete: %d %v", res.StatusCode, deleted)
	}
}
