package deepread

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// A scan stopped by a restart goes on after the part it last finished: that part is not read again.
func TestScanResumesAfterTheLastFinishedPart(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	ai := fakeAI(t, true)
	var mu sync.Mutex
	var bodies []string
	spy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(raw))
		mu.Unlock()
		res, err := http.Post(ai.URL+r.URL.Path, "application/json", bytes.NewReader(raw))
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		defer res.Body.Close()
		_, _ = io.Copy(w, res.Body)
	}))
	defer spy.Close()
	_ = st.SetSetting(store.KeyLLMBaseURL, spy.URL)
	_ = st.SetSetting(store.KeyLLMModel, "big-model")
	calibreDir := t.TempDir()
	path := writeBook(t, calibreDir)
	cat, _ := st.EnsureCatalog("Calibre Main", "calibre")
	id, _ := st.UpsertBook("Mixed Book", "Author", "", "")
	_ = st.AddCopy(cat, id, path, "epub", "5")
	two := 2
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &two, Model: "gpt"})
	r := New(st, calibreDir)
	_ = st.SetSetting(store.KeyLocalContext, "32768")
	if n, _ := r.Queue([]int64{id}, "admin", "admin"); n != 1 {
		t.Fatal("not queued")
	}
	next, _ := st.NextDeepRead()
	parts, _, err := r.Prepare(id)
	if err != nil || len(parts) < 2 {
		t.Fatalf("parts: %d %v", len(parts), err)
	}
	// as if the first part had been read before the restart
	raw, _ := json.Marshal([]savedPart{{Label: parts[0].Label, Result: llm.PartResult{Level: 1, Note: "kept"}}})
	st.SaveDeepProgress(next.ID, string(raw))
	if err := r.scan(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	first, later := false, false
	for _, b := range bodies {
		first = first || strings.Contains(b, "Part 1 of")
		later = later || strings.Contains(b, "Part 2 of")
	}
	if first || !later {
		t.Fatalf("part 1 read again: %v, part 2 read: %v", first, later)
	}
	if st.DeepProgress(next.ID) != "" {
		t.Fatal("progress should be cleared when the scan ends")
	}
}
