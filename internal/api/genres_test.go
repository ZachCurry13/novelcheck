package api_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestFillGenresWithAI(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		// Every book in the batch comes back as fantasy fiction.
		var books []string
		for _, m := range regexp.MustCompile(`(\d+)\. `).FindAllStringSubmatch(string(raw), -1) {
			books = append(books, fmt.Sprintf(`{"id": %s, "genres": ["fantasy", "nonsense"], "fiction": true}`, m[1]))
		}
		answer := `{"books": [` + strings.Join(books, ",") + `]}`
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": answer}}},
			"usage":   map[string]int{"prompt_tokens": 500, "completion_tokens": 60}})
	}))
	defer fake.Close()
	srv, st := setup(t)
	_ = st.SetSetting(store.KeyLLMBaseURL, fake.URL)
	_ = st.SetSetting(store.KeyLLMModel, "small-model")
	admin := login(t, srv, "admin", "adminpass1")
	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	for i, b := range []struct {
		title string
		tags  []string
	}{{"The Hobbit", nil}, {"Mort", nil}, {"Team of Rivals", []string{"History", "Nonfiction"}}} {
		id, _ := st.UpsertCalibreBook(cat, store.CalibreEntry{ExtID: fmt.Sprint(i + 1), Title: b.title, Tags: b.tags})
		_ = st.AddCalibreCopy(cat, id, fmt.Sprintf("/calibre/%d.epub", i), "epub", fmt.Sprint(i+1), "")
	}

	_, s := admin.do("GET", "/api/admin/status", nil, false)
	if g := s["genres"].(map[string]any); g["missing"].(float64) != 2 || g["running"] != false {
		t.Fatalf("status: %v", g)
	}
	if res, _ := admin.do("POST", "/api/admin/genres/fill", nil, true); res.StatusCode != 200 {
		t.Fatalf("start: %d", res.StatusCode)
	}
	deadline := time.Now().Add(5 * time.Second)
	for st.CountMissingGenres() > 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	_, out := admin.do("GET", "/api/books?genre=fantasy", nil, false)
	if out["total"].(float64) != 2 {
		t.Fatalf("filled in: %v", out)
	}
	_, out = admin.do("GET", "/api/books?genre=history", nil, false)
	books := out["books"].([]any)
	if len(books) != 1 || books[0].(map[string]any)["genre_source"] != "calibre" {
		t.Fatalf("Calibre's own tags are left alone: %v", out)
	}
	if res, _ := admin.do("POST", "/api/admin/genres/fill", nil, true); res.StatusCode != 400 {
		t.Fatal("nothing left to fill")
	}
}
