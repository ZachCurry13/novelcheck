package analyzer_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// fakeLLM answers garbage for the small model so the worker must fall back.
func fakeLLM(calls *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var req struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		content := "I cannot answer that."
		if req.Model == "big-model" {
			content = `{"classification":"No Spice","content_elements":{},"spiritual_elements":{"playful_fantasy":true},
				"lgbtq_content":false,"summary_verdict":"Clean, whimsical adventure."}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": content}}},
			"usage":   map[string]int{"prompt_tokens": 500, "completion_tokens": 50},
		})
	}))
}

func TestWorkerAnalyzesWithFallback(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	var calls atomic.Int32
	srv := fakeLLM(&calls)
	defer srv.Close()
	for k, v := range map[string]string{
		store.KeyLLMBaseURL: srv.URL, store.KeyLLMModel: "small-model",
		store.KeyLLMFallbackModel: "big-model", store.KeyScanDelaySeconds: "0",
	} {
		_ = st.SetSetting(k, v)
	}
	id, _ := st.UpsertBook("The Hobbit", "J.R.R. Tolkien", "", "")
	// A pre-filled blurb skips the network enrichment step.
	_ = st.SetBlurb(id, "Bilbo Baggins is swept into a quest to reclaim a dwarven kingdom from a dragon.")

	w := analyzer.New(st)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	w.Enqueue(true, id)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		b, _ := st.BookByID(id, nil)
		if b.Status == "analyzed" {
			if *b.Classification != "No Spice" || !b.PlayfulFantasy || b.AnalysisModel != "big-model" {
				t.Fatalf("unexpected analysis %+v", b)
			}
			if calls.Load() != 2 {
				t.Fatalf("expected 2 LLM calls (small then fallback), got %d", calls.Load())
			}
			u, _ := st.Usage()
			if u.TotalCalls != 2 || u.LastHourTokens != 1100 {
				t.Fatalf("usage not recorded: %+v", u)
			}
			return
		}
		if b.Status == "error" {
			t.Fatalf("analysis failed: %s", b.AnalysisError)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for analysis")
}
