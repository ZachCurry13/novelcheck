package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// fakeOllamaTags lists installed models with their sizes.
func fakeOllamaTags(t *testing.T, models map[string]string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var list []string
		for name, params := range models {
			list = append(list, fmt.Sprintf(`{"name": %q, "size": 1, "details": {"parameter_size": %q}}`, name, params))
		}
		fmt.Fprintf(w, `{"models": [%s]}`, strings.Join(list, ","))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// The small-model warning checks the machine that really does Deep Scans,
// and an admin can keep a small model on purpose.
func TestDeepModelWarning(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	main := fakeOllamaTags(t, map[string]string{"qwen2.5:3b": "3.1B", "qwen2.5:7b": "7.6B"})
	big := fakeOllamaTags(t, map[string]string{"qwen2.5:7b": "7.6B", "qwen2.5:3b": "3.1B"})
	_ = st.SetSetting(store.KeyLLMBaseURL, main.URL+"/v1")
	_ = st.SetSetting(store.KeyLLMModel, "qwen2.5:3b")
	warn := func() map[string]any {
		_, out := admin.do("GET", "/api/admin/deep-scans", nil, false)
		return out
	}

	if out := warn(); !strings.Contains(out["model_warning"].(string), "qwen2.5:3b") || out["suggest_model"] != "qwen2.5:7b" {
		t.Fatalf("a small main model: %v %v", out["model_warning"], out["suggest_model"])
	}

	// A Deep Scan machine with a 7B model reads the books: no warning.
	for k, v := range map[string]string{store.KeyDeepEnabled: "true", store.KeyDeepProvider: "openai",
		store.KeyDeepBaseURL: big.URL + "/v1", store.KeyDeepModels: "qwen2.5:7b"} {
		_ = st.SetSetting(k, v)
	}
	if out := warn(); out["model_warning"] != "" {
		t.Fatalf("the Deep Scan machine uses a 7B model: %v", out["model_warning"])
	}

	// A small model on that machine: warned, pointing at AI machines, and no
	// one-tap switch (it would change the main AI's Deep Scan model).
	_ = st.SetSetting(store.KeyDeepModels, "qwen2.5:3b")
	out := warn()
	if w := out["model_warning"].(string); !strings.Contains(w, "Deep Scan machine") || out["suggest_model"] != "" || out["warned_model"] != "qwen2.5:3b" {
		t.Fatalf("a small Deep Scan machine model: %v", out)
	}

	// Keep using it: the warning goes away for that model.
	if res, _ := admin.do("PUT", "/api/admin/settings", map[string]string{"deep_model_ok": "qwen2.5:3b"}, true); res.StatusCode != 200 {
		t.Fatalf("keep: %d", res.StatusCode)
	}
	if out := warn(); out["model_warning"] != "" {
		t.Fatalf("kept on purpose: %v", out["model_warning"])
	}
}
