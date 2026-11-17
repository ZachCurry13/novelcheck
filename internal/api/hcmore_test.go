package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The book page's Hardcover section: the audiobook (length, narrator) and "More like this", most similar first,
// a family book marked and opened from NovelCheck, its covers passed through NovelCheck; a kid sees only the
// family's books.
func TestHardcoverMoreLikeThis(t *testing.T) {
	asked := 0
	hc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var q struct{ Query string }
		_ = json.Unmarshal(raw, &q)
		switch {
		case strings.Contains(q.Query, "search("):
			io.WriteString(w, `{"data": {"search": {"ids": [1]}}}`)
		case strings.Contains(q.Query, "_in: [1]"):
			io.WriteString(w, `{"data": {"books": [{"id": 1, "title": "Dragon Book", "contributions": [{"author": {"name": "Ann Author"}}]}]}}`)
		case strings.Contains(q.Query, "_eq: 1"):
			asked++
			io.WriteString(w, `{"data": {"books": [{"audio_seconds": 100, "cached_similar_book_ids": [3, {"id": 2}],
				"default_audio_edition": {"asin": "B00TEST", "audio_seconds": 36000,
				"cached_contributors": [{"author": {"name": "Ann Author"}}, {"author": {"name": "Ned Narrator"}, "contribution": "Narrator"}]}}]}}`)
		case strings.Contains(q.Query, "_in: [3, 2]"):
			io.WriteString(w, `{"data": {"books": [
				{"id": 2, "title": "Owned Sequel", "rating": 4.1, "contributions": [{"author": {"name": "Ann Author"}}]},
				{"id": 3, "title": "Shop Book", "rating": 3.8, "cached_image": {"url": "https://assets.hardcover.app/x.jpg"}, "contributions": [{"author": {"name": "Bo Writer"}}]}]}}`)
		default:
			io.WriteString(w, `{"data": {"me": [{"id": 7, "username": "reader7"}]}}`)
		}
	}))
	defer hc.Close()
	api.SetHardcoverURL(hc.URL)
	t.Cleanup(func() { api.SetHardcoverURL("") })
	srv, st := setupWith(t, nil)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	book, _ := st.UpsertBook("Dragon Book", "Ann Author", "", "")
	sequel, _ := st.UpsertBook("Owned Sequel", "Ann Author", "", "")
	_ = st.AddCopy(cal, book, "/c/a.epub", "epub", "1")
	_ = st.AddCopy(cal, sequel, "/c/b.epub", "epub", "2")
	zero := 0
	for _, id := range []int64{book, sequel} { // rated, so a kid may open them
		_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &zero, ContentSource: store.SourceAI, Classification: "None"})
	}

	if _, out := admin.do("GET", "/api/books/"+itoa(book)+"/hardcover", nil, false); out["similar"] != nil && len(out["similar"].([]any)) != 0 {
		t.Fatalf("nobody connected Hardcover: %v", out)
	}
	admin.do("PUT", "/api/me/hardcover", map[string]string{"token": "good-token"}, true)
	res, out := admin.do("GET", "/api/books/"+itoa(book)+"/hardcover", nil, false)
	if res.StatusCode != 200 || out["audio_seconds"] != 36000.0 || out["asin"] != "B00TEST" {
		t.Fatalf("the audiobook: %d %v", res.StatusCode, out)
	}
	if n := out["narrators"].([]any); len(n) != 1 || n[0] != "Ned Narrator" {
		t.Fatalf("narrators: %v", n)
	}
	sim := out["similar"].([]any)
	if len(sim) != 2 {
		t.Fatalf("two similar books: %v", sim)
	}
	first, second := sim[0].(map[string]any), sim[1].(map[string]any)
	if first["title"] != "Shop Book" || first["book_id"] != nil || first["cover"] != "/api/libby/cover?u=https%3A%2F%2Fassets.hardcover.app%2Fx.jpg" {
		t.Fatalf("most similar first, its cover through NovelCheck: %v", first)
	}
	if second["book_id"] != float64(sequel) || second["cover"] != "/api/books/"+itoa(sequel)+"/cover" {
		t.Fatalf("the family's copy: %v", second)
	}
	admin.do("GET", "/api/books/"+itoa(book)+"/hardcover", nil, false)
	if asked != 1 {
		t.Fatalf("kept for a day: asked %d times", asked)
	}

	admin.do("POST", "/api/admin/users", map[string]string{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	_, out = kid.do("GET", "/api/books/"+itoa(book)+"/hardcover", nil, false)
	if sim := out["similar"].([]any); len(sim) != 1 || sim[0].(map[string]any)["title"] != "Owned Sequel" {
		t.Fatalf("a kid sees only the family's books: %v", sim)
	}
}
