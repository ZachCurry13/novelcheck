package api_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// "Try the new rules on 20 books" rates a sample with the main AI, shows the answers beside the ratings the books
// have, and saves nothing.
func TestTrialRatesASampleAndSavesNothing(t *testing.T) {
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		verdict, _ := json.Marshal(map[string]any{"spice_level": 3, "spice_reason": "Heavy innuendo", "content_elements": map[string]bool{},
			"spiritual_elements": map[string]bool{}, "content": []string{}, "premise": "A book.", "confidence": "high", "summary_verdict": "Fine."})
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": string(verdict)}}}})
	}))
	defer ai.Close()
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	_ = st.SetSettings(map[string]string{store.KeyLLMProvider: "openai", store.KeyLLMBaseURL: ai.URL + "/v1", store.KeyLLMModel: "llama3.2", store.KeyLLMAPIKey: "x"})
	if res, _ := admin.do("POST", "/api/admin/rerate/trial", map[string]int{"count": 6}, true); res.StatusCode != 400 {
		t.Fatalf("nothing rated yet: %d", res.StatusCode)
	}
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	for i := 0; i < 8; i++ {
		id, _ := st.UpsertBook(fmt.Sprintf("Sample Book %d", i), "Pat Writer", "", "A blurb that is long enough.")
		_ = st.AddCopy(cal, id, fmt.Sprintf("/c/%d.epub", i), "epub", fmt.Sprint(i))
		level := i % 4 // 0, 1, 2, 3
		_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &level, Confidence: "high", ContentSource: store.SourceAI, Model: "llama3.2"})
	}
	if res, _ := admin.do("POST", "/api/admin/rerate/trial", map[string]int{"count": 6}, true); res.StatusCode != 202 {
		t.Fatalf("start: %d", res.StatusCode)
	}
	var out map[string]any
	for deadline := time.Now().Add(20 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		_, out = admin.do("GET", "/api/admin/rerate/trial", nil, false)
		if out["running"] == false {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the trial never finished: %v", out)
		}
	}
	rows := out["results"].([]any)
	if len(rows) != 6 {
		t.Fatalf("six books tried: %d", len(rows))
	}
	higher, lower, same := 0, 0, 0
	for _, r := range rows {
		m := r.(map[string]any)
		if m["new_level"] != 3.0 || m["new_confidence"] == "" || m["new_confidence"] == nil {
			t.Fatalf("the AI's new answer: %v", m)
		}
		switch m["difference"] {
		case "higher":
			higher++
		case "lower":
			lower++
		default:
			same++
		}
	}
	if higher == 0 || same == 0 || lower != 0 {
		t.Fatalf("books rated 0 to 2 come out higher, the 3s the same: %d higher, %d same, %d lower", higher, same, lower)
	}
	var changed int
	_ = st.DB.Get(&changed, `SELECT COUNT(*) FROM books WHERE spice_level = 3 AND id IN (SELECT id FROM books WHERE title LIKE 'Sample Book %')`)
	if changed != 2 { // i = 3 and i = 7 were 3 already; nothing was saved for the rest
		t.Fatalf("nothing was saved: %d books at level 3", changed)
	}
}
