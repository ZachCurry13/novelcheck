package analyzer_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// A photo goes to the local photo model first; without one, a model that says it can't take photos is asked
// once and then skipped.
func TestPhotoModelFirstAndBlindModelsSkipped(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	var mu sync.Mutex
	var asked []string
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(raw, &req)
		mu.Lock()
		asked = append(asked, req.Model)
		mu.Unlock()
		if req.Model != "gemma3:4b" {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"error": {"message": "Multimodal data provided, but model does not support multimodal requests."}}`)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": `{"title": "Cinder", "author": "Marissa Meyer", "isbn": ""}`}}},
			"usage":   map[string]int{"prompt_tokens": 10, "completion_tokens": 10},
		})
	}))
	defer local.Close()
	_ = st.SetSetting(store.KeyLLMBaseURL, local.URL+"/v1")
	_ = st.SetSetting(store.KeyLLMModel, "llama3.2:latest")
	w := analyzer.New(st)
	img := []byte("jpeg")

	if _, err := w.ReadCover(context.Background(), img, "image/jpeg"); err == nil || !strings.Contains(err.Error(), "Photo-reading model") {
		t.Fatalf("with no model that can see, the error says what to set: %v", err)
	}
	if _, err := w.ReadCover(context.Background(), img, "image/jpeg"); err == nil {
		t.Fatal("still no model that can see")
	}
	mu.Lock()
	n := len(asked)
	mu.Unlock()
	if n != 1 {
		t.Fatalf("a model that can't take photos is asked once, then skipped: %v", asked)
	}

	_ = st.SetSetting(store.KeyVisionModel, "gemma3:4b")
	f, err := w.ReadCover(context.Background(), img, "image/jpeg")
	if err != nil || f.Title != "Cinder" || f.Author != "Marissa Meyer" {
		t.Fatalf("the photo model reads it: %+v %v", f, err)
	}
	mu.Lock()
	last := asked[len(asked)-1]
	total := len(asked)
	mu.Unlock()
	if last != "gemma3:4b" || total != 2 {
		t.Fatalf("the photo model is asked first, and nothing else after it answers: %v", asked)
	}
}

// A reading that can't be confirmed as a real book is not taken at its word: the next AI reads the photo too,
// and its confirmed answer wins. With nothing confirmed, the first reading comes back marked unsure.
func TestPhotoReadingIsConfirmed(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	answer := func(title string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			if strings.Contains(string(raw), "llama3.2") { // the text-only model on the same machine
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, `{"error": {"message": "Multimodal data provided, but model does not support multimodal requests."}}`)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": `{"title": "` + title + `", "author": "Marissa Meyer"}`}}},
				"usage":   map[string]int{"prompt_tokens": 10, "completion_tokens": 10},
			})
		}))
	}
	small, big := answer("Cider"), answer("Cinder")
	defer small.Close()
	defer big.Close()
	_ = st.SetSetting(store.KeyLLMBaseURL, small.URL+"/v1")
	_ = st.SetSetting(store.KeyLLMModel, "llama3.2:latest")
	_ = st.SetSetting(store.KeyVisionModel, "gemma3:4b")
	_ = st.SetSetting(store.KeyBackupBaseURL, big.URL+"/v1")
	_ = st.SetSetting(store.KeyBackupModel, "big-model")
	_ = st.SetSetting(store.KeyBackupEnabled, "true")
	w := analyzer.New(st)
	real := func(f enrich.Found) bool { return f.Title == "Cinder" }

	f, sure, err := w.ReadCoverChecked(context.Background(), []byte("jpeg"), "image/jpeg", real)
	if err != nil || !sure || f.Title != "Cinder" {
		t.Fatalf("the confirmed reading wins: %+v sure=%v %v", f, sure, err)
	}
	f, sure, err = w.ReadCoverChecked(context.Background(), []byte("jpeg"), "image/jpeg", func(enrich.Found) bool { return false })
	if err != nil || sure || f.Title != "Cider" {
		t.Fatalf("nothing confirmed: the first reading, unsure: %+v sure=%v %v", f, sure, err)
	}
}
