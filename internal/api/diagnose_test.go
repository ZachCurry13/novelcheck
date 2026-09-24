package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiagnoseWithAI(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	var gotPrompt string
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotPrompt = req.Messages[len(req.Messages)-1].Content
		answer := `{"diagnosis":"Gmail rejected the login.","fix_steps":["Create an App Password"],"title":"Gmail login rejected","summary":"SMTP 535.","suspected_cause":"normal password used","is_bug":false}`
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": answer}}},
			"usage":   map[string]int{"prompt_tokens": 900, "completion_tokens": 80},
		})
	}))
	defer ai.Close()
	if res, out := admin.do("PUT", "/api/admin/settings", map[string]string{"llm_base_url": ai.URL, "llm_model": "m", "llm_api_key": "sk-hidden"}, true); res.StatusCode != 200 {
		t.Fatalf("settings: %d %v", res.StatusCode, out)
	}
	_ = st.SetSetting("tunnel_hostname", "books.example.net")

	res, d := admin.do("POST", "/api/admin/diagnose", map[string]string{"problem": "Test email fails at books.example.net"}, true)
	if res.StatusCode != 200 || d["title"] != "Gmail login rejected" || d["is_bug"] != false {
		t.Fatalf("diagnose: %d %v", res.StatusCode, d)
	}
	report := d["report"].(string)
	for _, want := range []string{"Gmail rejected the login.", "1. Create an App Password", "Diagnostics (no passwords or keys)", "your-address.example.com"} {
		if !strings.Contains(report, want) {
			t.Errorf("report missing %q", want)
		}
	}
	if strings.Contains(report, "books.example.net") || strings.Contains(report, "sk-hidden") || strings.Contains(gotPrompt, "sk-hidden") {
		t.Fatal("report or prompt leaked the hostname or the API key")
	}

	// With the AI down, a bug report is still produced.
	ai.Close()
	res, d = admin.do("POST", "/api/admin/diagnose", nil, true)
	if res.StatusCode != 200 || d["ai_error"] == "" || !strings.Contains(d["report"].(string), "AI diagnosis unavailable") {
		t.Fatalf("AI down: %d %v", res.StatusCode, d)
	}
}
