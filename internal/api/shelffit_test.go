package api_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/collections"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// shelfAI plans a search for "saint", picks every book it's shown, and in
// the second look says no to the football book.
func shelfAI(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body := string(raw)
		var ids []int64
		for _, line := range strings.Split(body, `\n`) {
			var id int64
			if _, err := fmt.Sscanf(line, "%d |", &id); err == nil {
				ids = append(ids, id)
			}
		}
		answer := `{"words": ["saint"], "genres": ["children"], "kind": ""}`
		var parts []string
		switch {
		case strings.Contains(body, "Shelf check"):
			for _, line := range strings.Split(body, `\n`) {
				var id int64
				if _, err := fmt.Sscanf(line, "%d |", &id); err == nil {
					fits := map[bool]string{true: "no", false: "yes"}[strings.Contains(line, "football")]
					parts = append(parts, fmt.Sprintf(`{"id": %d, "fits": %q, "reason": "checked"}`, id, fits))
				}
			}
			answer = `{"answers": [` + strings.Join(parts, ",") + `]}`
		case strings.Contains(body, "Books (id"):
			for _, id := range ids {
				parts = append(parts, fmt.Sprintf(`{"id": %d, "fit": "clearly", "reason": "fits"}`, id))
			}
			answer = `{"picks": [` + strings.Join(parts, ",") + `]}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": answer}}},
			"usage":   map[string]int{"prompt_tokens": 100, "completion_tokens": 20}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func shelfTitles(t *testing.T, admin *client, q string) []string {
	t.Helper()
	_, out := admin.do("GET", "/api/books?"+q, nil, false)
	var titles []string
	for _, b := range out["books"].([]any) {
		titles = append(titles, b.(map[string]any)["title"].(string))
	}
	return titles
}

func waitShelfJob(t *testing.T, admin *client, id string) map[string]any {
	t.Helper()
	for i := 0; i < 200; i++ {
		if _, j := admin.do("GET", "/api/collections/ai/"+id, nil, false); j["status"] != "running" {
			return j
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job never finished")
	return nil
}

func TestShelvesStayOnTheme(t *testing.T) {
	srv, st := setupWith(t, func(s *api.Server) { s.Collections = collections.New(s.Store) })
	ai := shelfAI(t)
	_ = st.SetSetting(store.KeyLLMBaseURL, ai.URL)
	_ = st.SetSetting(store.KeyLLMModel, "model")
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	ids := map[string]int64{}
	for i, b := range [][2]string{
		{"A Canticle for Leibowitz", "Monks keep the relics of Saint Leibowitz after the war."},
		{"The Bad Beginning", "The unhappy lives of the Baudelaire orphans."},
		{"Brother Francis", "The life of a saint who loved animals."},
		{"Game Day", "A football season with the New Orleans Saints."},
	} {
		id, _ := st.UpsertBook(b[0], "Author", "", "")
		_, _ = st.DB.Exec(`UPDATE books SET description = ? WHERE id = ?`, b[1], id)
		_ = st.AddCopy(cal, id, fmt.Sprintf("/c/%d.epub", i), "epub", fmt.Sprint(i+1))
		ids[b[0]] = id
	}

	// Whole words: the Baudelaire orphans' "lives of the" no longer counts.
	// (Title order skips "A".)
	if got := strings.Join(shelfTitles(t, admin, "season=saints"), "|"); got != "Brother Francis|A Canticle for Leibowitz|Game Day" {
		t.Fatalf("saints shelf: %s", got)
	}

	// A faith shelf leaves out books flagged Dark Occult, whatever their words.
	occult, _ := st.UpsertBook("The Haunted Abbey", "Author", "", "")
	_, _ = st.DB.Exec(`UPDATE books SET description = 'A saint fights demons.', dark_occult = 1 WHERE id = ?`, occult)
	_ = st.AddCopy(cal, occult, "/c/occult.epub", "epub", "99")
	_, _ = st.DB.Exec(`DELETE FROM season_books`) // the shelf's matches are remade on the next visit
	for _, title := range shelfTitles(t, admin, "season=saints") {
		if title == "The Haunted Abbey" {
			t.Fatal("a Dark Occult book on the Saints shelf")
		}
	}

	// Not for this shelf: gone at once, and remembered.
	res, out := admin.do("POST", "/api/shelves/reject", map[string]any{"season": "saints", "ids": []int64{ids["Brother Francis"]}}, true)
	if res.StatusCode != 200 || out["removed"].(float64) != 1 {
		t.Fatalf("reject: %d %v", res.StatusCode, out)
	}
	if got := strings.Join(shelfTitles(t, admin, "season=saints"), "|"); got != "A Canticle for Leibowitz|Game Day" {
		t.Fatalf("after reject: %s", got)
	}

	// Check these books: the AI's second look finds the football book.
	res, out = admin.do("POST", "/api/shelves/check", map[string]any{"season": "saints"}, true)
	if res.StatusCode != 202 || out["total"].(float64) != 2 {
		t.Fatalf("check: %d %v", res.StatusCode, out)
	}
	job := waitShelfJob(t, admin, out["job"].(string))
	misfits := job["picks"].([]any)
	if job["status"] != "done" || len(misfits) != 1 || misfits[0].(map[string]any)["book"].(map[string]any)["title"] != "Game Day" {
		t.Fatalf("check job: %v", job)
	}

	// Build with AI skips the rejected book, and the second look drops football.
	res, out = admin.do("POST", "/api/collections/ai", map[string]any{"theme": "lives of the saints", "season": "saints"}, true)
	if res.StatusCode != 202 {
		t.Fatalf("fill: %d %v", res.StatusCode, out)
	}
	job = waitShelfJob(t, admin, out["job"].(string))
	var picked []string
	for _, p := range job["picks"].([]any) {
		picked = append(picked, p.(map[string]any)["book"].(map[string]any)["title"].(string))
	}
	if strings.Join(picked, "|") != "A Canticle for Leibowitz" {
		t.Fatalf("fill picks: %v (%v)", picked, job)
	}

	// Once built, the shelf is the collection; taking a book off it removes it
	// there and keeps it off the words shelf too.
	res, out = admin.do("POST", "/api/collections", map[string]any{"name": "Saints", "theme": "lives of the saints", "season": "saints",
		"ids": []int64{ids["A Canticle for Leibowitz"], ids["Game Day"]}}, true)
	if res.StatusCode != 200 && res.StatusCode != 201 {
		t.Fatalf("create: %d %v", res.StatusCode, out)
	}
	admin.do("POST", "/api/shelves/reject", map[string]any{"season": "saints", "ids": []int64{ids["Game Day"]}}, true)
	if got := strings.Join(shelfTitles(t, admin, "season=saints"), "|"); got != "A Canticle for Leibowitz" {
		t.Fatalf("collection shelf after reject: %s", got)
	}
	if n := len(st.ShelfRejects(0, "saints")); n != 2 {
		t.Fatalf("remembered rejects: %d", n)
	}
}
