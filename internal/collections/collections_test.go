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

// fakeAI answers the plan with "dragon" (and the children's genre, which
// mustn't bring in every book), then picks every listed book with "Dragon"
// in its title, plus Plain Book 1 "loosely" and Plain Book 2 "clearly" (the
// second look says no to it) and an id that wasn't offered: all three are
// dropped. The second look says yes only to dragon books.
func fakeAI(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body := string(raw)
		answer := `{"words": ["dragon", "plain book 1", "plain book 2"], "genres": ["children"], "kind": ""}`
		lines := func(each func(id int64, line string) string) string {
			var out []string
			for _, line := range strings.Split(body, `\n`) {
				var id int64
				if _, err := fmt.Sscanf(line, "%d |", &id); err == nil && strings.Contains(line, " | ") {
					if s := each(id, line); s != "" {
						out = append(out, s)
					}
				}
			}
			return strings.Join(out, ",")
		}
		switch {
		case strings.Contains(body, "suggest themed book collections"):
			answer = `{"ideas": [{"name": "Dragon tales", "icon": "🐉", "theme": "stories with dragons"}]}`
		case strings.Contains(body, "Shelf check"):
			answer = `{"answers": [` + lines(func(id int64, line string) string {
				if strings.Contains(line, "Dragon") {
					return fmt.Sprintf(`{"id": %d, "fits": "yes", "reason": "dragons"}`, id)
				}
				return fmt.Sprintf(`{"id": %d, "fits": "no", "reason": "no dragons in it"}`, id)
			}) + `]}`
		case strings.Contains(body, "Books (id"):
			picks := lines(func(id int64, line string) string {
				switch {
				case strings.Contains(line, "Dragon"):
					return fmt.Sprintf(`{"id": %d, "fit": "clearly", "reason": "a dragon story"}`, id)
				case strings.Contains(line, "Plain Book 1 |"):
					return fmt.Sprintf(`{"id": %d, "fit": "loosely", "reason": "a book"}`, id)
				case strings.Contains(line, "Plain Book 2 |"):
					return fmt.Sprintf(`{"id": %d, "fit": "clearly", "reason": "sounds right"}`, id)
				}
				return ""
			})
			answer = `{"picks": [` + picks + `, {"id": 99999, "fit": "clearly", "reason": "not offered"}]}`
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

func TestCheckShelf(t *testing.T) {
	st := setup(t)
	books, err := st.ThemeCandidates([]string{"book"}, nil, "", nil, 100)
	if err != nil || len(books) != 40 {
		t.Fatalf("books %d: %v", len(books), err)
	}
	s := New(st)
	id, err := s.StartCheck("stories with dragons", books)
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		j := s.Job(id)
		if j != nil && j.Status == "done" {
			if len(j.Picks) != 35 || j.Done != 40 || j.Total != 40 || j.Picks[0].Reason != "no dragons in it" {
				t.Fatalf("misfits %d, done %d/%d: %+v", len(j.Picks), j.Done, j.Total, j.Picks[0])
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("job: %+v", s.Job(id))
		}
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
