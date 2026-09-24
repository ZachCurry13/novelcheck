package analyzer_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// With the main Ollama switched off, the backup AI rates the book, the main
// server's other models aren't tried, and admins get a notice.
func TestBackupAIWhenMainIsDown(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)

	// A port with nothing listening: "connection refused".
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	down := "http://" + ln.Addr().String() + "/v1"
	ln.Close()

	var backupCalls atomic.Int32
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backupCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"role": "assistant",
				"content": `{"spice_level": 1, "summary_verdict": "Sweet."}`}}},
			"usage": map[string]int{"prompt_tokens": 1000000, "completion_tokens": 0},
		})
	}))
	defer backup.Close()

	for k, v := range map[string]string{
		store.KeyLLMBaseURL: down, store.KeyLLMModel: "qwen2.5:7b", store.KeyLLMFallbackModel: "llama3.2",
		store.KeyPriceInputPerM: "0", store.KeyScanDelaySeconds: "0",
		store.KeyBackupEnabled: "true", store.KeyBackupBaseURL: backup.URL, store.KeyBackupModel: "gpt-4o-mini",
		store.KeyBackupPriceIn: "0.15",
	} {
		_ = st.SetSetting(k, v)
	}
	id, _ := st.UpsertBook("Backup Book", "A", "", "")
	_ = st.SetBlurb(id, "x")
	w := analyzer.New(st)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	w.Enqueue(true, id)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		b, _ := st.BookByID(id, nil)
		if b.Status == "error" {
			t.Fatalf("backup should have rated it: %s", b.AnalysisError)
		}
		if b.Status == "analyzed" {
			if b.AnalysisModel != "gpt-4o-mini" || *b.SpiceLevel != 1 || backupCalls.Load() != 1 {
				t.Fatalf("rated by %s, level %v, backup calls %d", b.AnalysisModel, b.SpiceLevel, backupCalls.Load())
			}
			if got := st.SpentUSD(); got < 0.149 || got > 0.151 {
				t.Fatalf("backup call should cost its own price ($0.15), got %v", got)
			}
			items, _, _ := st.Notifications(10)
			if len(items) != 1 || items[0].Source != "llm-backup" {
				t.Fatalf("expected a backup notice, got %+v", items)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("book was not rated")
}

func TestUnreachable(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()
	_, err := http.Get("http://" + addr)
	if !analyzer.Unreachable(err) {
		t.Fatalf("connection refused should count as unreachable: %v", err)
	}
	if analyzer.Unreachable(context.DeadlineExceeded) {
		t.Fatal("a timeout is not 'unreachable'")
	}
}
