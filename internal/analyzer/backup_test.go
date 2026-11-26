package analyzer_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/llm"
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

// When the main AI has used up its free requests for today, the backup's takeover is a calm note, not a warning.
func TestBackupAfterDailyLimitIsANote(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	main := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"Generate content requests per day limit exceeded on Free Tier. Please retry in 4h28m24s."}}`))
	}))
	defer main.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{
			"role": "assistant", "content": `{"spice_level": 1, "summary_verdict": "Sweet."}`}}}})
	}))
	defer backup.Close()
	for k, v := range map[string]string{
		store.KeyLLMBaseURL: main.URL, store.KeyLLMModel: "gemini-a", store.KeyScanDelaySeconds: "0",
		store.KeyBackupEnabled: "true", store.KeyBackupBaseURL: backup.URL, store.KeyBackupModel: "llama",
	} {
		_ = st.SetSetting(k, v)
	}
	id, _ := st.UpsertBook("Limit Book", "A", "", "")
	_ = st.SetBlurb(id, "x")
	w := analyzer.New(st)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	w.Enqueue(true, id)
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if b, _ := st.BookByID(id, nil); b.Status == "analyzed" {
			items, _, _ := st.Notifications(10)
			if len(items) != 1 || items[0].Level != "info" || !strings.Contains(items[0].Message, "free limit for today") ||
				!strings.Contains(items[0].Message, "the local AI") {
				t.Fatalf("expected a calm note, got %+v", items)
			}
			return
		}
	}
	t.Fatal("book was not rated")
}

// With a person waiting, two "busy" answers from Google skip its next model (Gemma's slow 30 s) for the backup,
// and the takeover is a calm note.
func TestBusyMainGoesStraightToBackup(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	var asked []string
	main := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		asked = append(asked, body.Model)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"message":"This model is currently experiencing high demand."}}`))
	}))
	defer main.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{
			"role": "assistant", "content": `{"spice_level": 1, "summary_verdict": "Sweet."}`}}}})
	}))
	defer backup.Close()
	for k, v := range map[string]string{
		store.KeyLLMBaseURL: main.URL, store.KeyLLMModel: "flash-a", store.KeyLLMFallbackModel: "flash-b,gemma", store.KeyScanDelaySeconds: "0",
		store.KeyBackupEnabled: "true", store.KeyBackupBaseURL: backup.URL, store.KeyBackupModel: "llama",
	} {
		_ = st.SetSetting(k, v)
	}
	id, _ := st.UpsertBook("Busy Book", "A", "", "")
	_ = st.SetBlurb(id, "x")
	if err := analyzer.New(st).RateNow(llm.Quick(context.Background()), id); err != nil {
		t.Fatal(err)
	}
	if strings.Join(asked, ",") != "flash-a,flash-b" {
		t.Fatalf("Gemma should be skipped after two busy answers: asked %v", asked)
	}
	items, _, _ := st.Notifications(10)
	if len(items) != 1 || items[0].Level != "info" || !strings.Contains(items[0].Message, "too busy") {
		t.Fatalf("expected a calm note, got %+v", items)
	}
}

// The first model says its free requests for today are used up and the next one fails for another reason (a name that no longer
// exists): the backup's takeover is still the calm note about the limit, and the same note, however many books it rates.
func TestLimitThenAnotherFaultIsStillTheLimitNote(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	main := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Model == "gemini-a" {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"Generate content requests per day limit exceeded on Free Tier. Please retry in 4h28m24s."}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"models/gemini-gone is not found"}}`))
	}))
	defer main.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{
			"role": "assistant", "content": `{"spice_level": 1, "summary_verdict": "Sweet."}`}}}})
	}))
	defer backup.Close()
	for k, v := range map[string]string{
		store.KeyLLMBaseURL: main.URL, store.KeyLLMModel: "gemini-a", store.KeyLLMFallbackModel: "gemini-gone", store.KeyScanDelaySeconds: "0",
		store.KeyBackupEnabled: "true", store.KeyBackupBaseURL: backup.URL, store.KeyBackupModel: "llama",
	} {
		_ = st.SetSetting(k, v)
	}
	var ids []int64
	for _, title := range []string{"One", "Two", "Three"} {
		id, _ := st.UpsertBook(title, "A", "", "")
		_ = st.SetBlurb(id, "x")
		ids = append(ids, id)
	}
	w := analyzer.New(st)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	w.Enqueue(true, ids...)
	for deadline := time.Now().Add(8 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		done := 0
		for _, id := range ids {
			if b, _ := st.BookByID(id, nil); b.Status == "analyzed" {
				done++
			}
		}
		if done == len(ids) {
			items, _, _ := st.Notifications(10)
			var notes []store.Notification
			for _, n := range items {
				if n.Source == "llm-backup" {
					notes = append(notes, n)
				}
			}
			if len(notes) != 1 || notes[0].Level != "info" || !strings.Contains(notes[0].Message, "free limit for today") || notes[0].Count != 3 {
				t.Fatalf("expected one calm note, counted three times, got %+v", notes)
			}
			return
		}
	}
	t.Fatal("books were not rated")
}
