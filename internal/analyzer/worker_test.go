package analyzer_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// fakeLLM answers garbage for every model but big-model, so the worker must
// work down its fallback list in order.
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
		store.KeyLLMFallbackModel: "mid-model, big-model", store.KeyScanDelaySeconds: "0",
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
			if calls.Load() != 3 {
				t.Fatalf("expected 3 LLM calls (small, mid, then big), got %d", calls.Load())
			}
			u, _ := st.Usage()
			if u.TotalCalls != 3 || u.LastHourTokens != 1650 {
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

func TestRerateKeepsBookVisible(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	var calls atomic.Int32
	srv := fakeLLM(&calls)
	defer srv.Close()
	_ = st.SetSetting(store.KeyLLMBaseURL, srv.URL)
	_ = st.SetSetting(store.KeyLLMModel, "big-model")
	id, _ := st.UpsertBook("The Hobbit", "J.R.R. Tolkien", "", "")
	_ = st.SetBlurb(id, "A hobbit goes on an adventure.")
	_ = st.SaveAnalysis(id, store.Analysis{Classification: "Closed Door", Model: "old-model"})

	w := analyzer.New(st)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if n := w.Rerate(id, id); n != 1 {
		t.Fatalf("rerate should de-duplicate, added %d", n)
	}
	if b, _ := st.BookByID(id, nil); b.Status != "analyzed" {
		t.Fatalf("queued re-rate must not hide the book: %s", b.Status)
	}
	go w.Run(ctx)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if b, _ := st.BookByID(id, nil); b.AnalysisModel == "big-model" {
			if b.Status != "analyzed" || *b.Classification != "No Spice" {
				t.Fatalf("re-rated book: %+v", b)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("re-rate did not finish")
}

func TestSlowModelGivesClearError(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer slow.Close()
	for k, v := range map[string]string{store.KeyLLMBaseURL: slow.URL, store.KeyLLMModel: "qwen2.5:7b",
		store.KeyLLMTimeoutSeconds: "1", store.KeyScanDelaySeconds: "0"} {
		_ = st.SetSetting(k, v)
	}
	id, _ := st.UpsertBook("Slow Book", "A", "", "")
	_ = st.SetBlurb(id, "x")
	w := analyzer.New(st)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	w.Enqueue(true, id)
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if b, _ := st.BookByID(id, nil); b.Status == "error" {
			if !strings.Contains(b.AnalysisError, "didn't answer within 1s") || !strings.Contains(b.AnalysisError, "GPU") {
				t.Fatalf("error should explain the timeout: %q", b.AnalysisError)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("slow model did not time out")
}
