package collections

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// fakeAI answers the plan with "dragon", then picks every listed book with
// "Dragon" in its title (plus an id that wasn't offered, which is dropped).
func fakeAI(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body := string(raw)
		answer := `{"words": ["dragon"], "genres": [], "kind": ""}`
		switch {
		case strings.Contains(body, "suggest themed book collections"):
			answer = `{"ideas": [{"name": "Dragon tales", "icon": "🐉", "theme": "stories with dragons"}]}`
		case strings.Contains(body, "Books (id"):
			var picks []string
			for _, line := range strings.Split(body, `\n`) {
				var id int64
				if strings.Contains(line, "Dragon") && strings.Contains(line, " | ") {
					fmt.Sscanf(line, "%d |", &id)
					picks = append(picks, fmt.Sprintf(`{"id": %d, "reason": "a dragon story"}`, id))
				}
			}
			picks = append(picks, `{"id": 99999, "reason": "not offered"}`)
			answer = `{"picks": [` + strings.Join(picks, ",") + `]}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": answer}}},
			"usage":   map[string]int{"prompt_tokens": 100, "completion_tokens": 20},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func setup(t *testing.T) *store.Store {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	st := store.New(d)
	ai := fakeAI(t)
	_ = st.SetSetting(store.KeyLLMBaseURL, ai.URL)
	_ = st.SetSetting(store.KeyLLMModel, "model")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	zero := 0
	for i := range 40 {
		title := fmt.Sprintf("Plain Book %d", i)
		if i%8 == 0 { // five dragon books
			title = fmt.Sprintf("Dragon Book %d", i)
		}
		id, _ := st.UpsertBook(title, "Author", "", "")
		_ = st.AddCopy(cal, id, "/c/"+title+".epub", "epub", title)
		_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &zero, ContentSource: store.SourceAI})
	}
	return st
}

func TestFillPicksMatchingBooks(t *testing.T) {
	st := setup(t)
	picks, err := Fill(context.Background(), st, "dragon stories", nil)
	if err != nil || len(picks) != 5 || picks[0].Reason != "a dragon story" {
		t.Fatalf("picks %+v: %v", picks, err)
	}
	// Find more skips books already in the collection.
	more, err := Fill(context.Background(), st, "dragon stories", []int64{picks[0].Book.ID, picks[1].Book.ID})
	if err != nil || len(more) != 3 {
		t.Fatalf("more %+v: %v", more, err)
	}
	s := New(st)
	id := s.Start("dragon stories", nil)
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if j := s.Job(id); j != nil && j.Status == "done" && len(j.Picks) == 5 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("job: %+v", s.Job(id))
		}
	}
	if _, err := Fill(context.Background(), st, " ", nil); err == nil {
		t.Fatal("an empty theme")
	}
}

func TestWeeklyIdeas(t *testing.T) {
	st := setup(t)
	s := New(st)
	s.Local = func() bool { return true }
	if !s.ideasDue(time.Now()) {
		t.Fatal("ideas should be due the first time")
	}
	if err := s.MakeIdeas(context.Background()); err != nil {
		t.Fatal(err)
	}
	cs, _ := st.ListCollections(nil)
	if len(cs) != 1 || cs[0].Kind != "idea" || cs[0].Name != "Dragon tales" || cs[0].Books != 5 || cs[0].Icon != "🐉" {
		t.Fatalf("ideas: %+v", cs)
	}
	if s.ideasDue(time.Now()) {
		t.Fatal("not due again within the week")
	}
	_ = st.SetSetting(store.KeyCollectionIdeas, "off")
	if s.IdeasOn() {
		t.Fatal("turned off")
	}
}
