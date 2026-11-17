package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The Audiobookshelf cleanup: a certain Hardcover match (by ISBN, or by ASIN with "(Unabridged)" on the title)
// is written into Audiobookshelf by itself; an ISBN that leads to a different title waits for a parent, who
// applies it; a book Hardcover doesn't know is left alone; nothing is looked at twice in a month.
func TestAudiobookshelfCleanup(t *testing.T) {
	hc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var q struct{ Query string }
		_ = json.Unmarshal(raw, &q)
		switch {
		case strings.Contains(q.Query, "isbn_13: {_in"):
			io.WriteString(w, `{"data": {"editions": [
				{"isbn_13": "9780000000011", "book": {"id": 10, "title": "Dune", "cached_image": {"url": "https://assets.hardcover.app/dune.jpg"},
				  "contributions": [{"author": {"name": "Frank Herbert"}}], "book_series": [{"position": 1, "series": {"name": "Dune"}}]}},
				{"isbn_13": "9780000000099", "book": {"id": 11, "title": "Other Book", "contributions": [{"author": {"name": "Someone Else"}}]}}]}}`)
		case strings.Contains(q.Query, "asin: {_in"):
			io.WriteString(w, `{"data": {"editions": [{"asin": "B003P2WO5E", "book": {"id": 12, "title": "The Way of Kings",
				"contributions": [{"author": {"name": "Brandon Sanderson"}}], "cached_tags": {"Genre": [{"tag": "Fantasy"}]}}}]}}`)
		case strings.Contains(q.Query, "_eq: 10"):
			io.WriteString(w, `{"data": {"books": [{"default_audio_edition": {"asin": "B00DUNE001", "audio_seconds": 1,
				"cached_contributors": [{"author": {"name": "Scott Brick"}, "contribution": "Narrator"}]}}]}}`)
		case strings.Contains(q.Query, "_eq:"):
			io.WriteString(w, `{"data": {"books": [{}]}}`)
		case strings.Contains(q.Query, "search("):
			io.WriteString(w, `{"data": {"search": {"ids": []}}}`)
		default:
			io.WriteString(w, `{"data": {"me": [{"id": 7, "username": "reader7"}]}}`)
		}
	}))
	defer hc.Close()
	var mu sync.Mutex
	writes := map[string]string{}
	abs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/libraries":
			fmt.Fprint(w, `{"libraries": [{"id": "lib1", "name": "Audiobooks", "mediaType": "book"}]}`)
		case r.URL.Path == "/api/libraries/lib1/items":
			fmt.Fprint(w, `{"total": 4, "results": [
			 {"id": "li_dune", "media": {"duration": 100, "metadata": {"title": "Dune", "authorName": "Frank Herbert", "isbn": "9780000000011", "genres": ["SF"], "description": "x"}}},
			 {"id": "li_wok", "media": {"duration": 100, "coverPath": "/c.jpg", "metadata": {"title": "The Way of Kings (Unabridged)", "authorName": "Brandon Sanderson",
			   "narratorName": "Michael Kramer", "asin": "B003P2WO5E", "description": "y"}}},
			 {"id": "li_bad", "media": {"duration": 100, "coverPath": "/c.jpg", "metadata": {"title": "Mislabelled", "authorName": "Ann Author", "isbn": "9780000000099",
			   "narratorName": "N", "asin": "B000000001", "genres": ["G"], "description": "z"}}},
			 {"id": "li_none", "media": {"duration": 100, "metadata": {"title": "Nobody Knows This", "authorName": "Al Unknown"}}}]}`)
		case r.Method == http.MethodPatch || r.Method == http.MethodPost:
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			writes[r.URL.Path] = string(b)
			mu.Unlock()
			fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer abs.Close()
	api.SetHardcoverURL(hc.URL)
	t.Cleanup(func() { api.SetHardcoverURL("") })
	var server *api.Server
	srv, st := setupWith(t, func(s *api.Server) {
		server = s
		s.Worker.NewEnricher = func() *enrich.Client {
			c := enrich.New("")
			c.OpenLibraryURL, c.GoogleBooksURL = hc.URL, hc.URL
			return c
		}
	})
	_ = st.SetSetting(store.KeyABSURL, abs.URL)
	_ = st.SetSetting(store.KeyABSToken, "tok")
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("PUT", "/api/me/hardcover", map[string]string{"token": "good-token"}, true)

	msg, err := server.CleanAudiobooks(context.Background(), 50)
	if err != nil || !strings.Contains(msg, "looked at 4 audiobooks, fixed 2, 1 waiting for you, 1 Hardcover doesn't know") {
		t.Fatalf("the round: %q %v", msg, err)
	}
	mu.Lock()
	dune, cover, wok := writes["/api/items/li_dune/media"], writes["/api/items/li_dune/cover"], writes["/api/items/li_wok/media"]
	_, bad := writes["/api/items/li_bad/media"]
	mu.Unlock()
	for _, want := range []string{`"series":[{"name":"Dune","sequence":"1"}]`, `"asin":"B00DUNE001"`, `"narrators":["Scott Brick"]`} {
		if !strings.Contains(dune, want) {
			t.Fatalf("Dune by ISBN: %s lacks %s", dune, want)
		}
	}
	if strings.Contains(dune, `"title"`) || cover != `{"url":"https://assets.hardcover.app/dune.jpg"}` {
		t.Fatalf("Dune's title stays, its cover comes from Hardcover: %s %s", dune, cover)
	}
	if !strings.Contains(wok, `"title":"The Way of Kings"`) || !strings.Contains(wok, `"genres":["Fantasy"]`) || strings.Contains(wok, "narrators") {
		t.Fatalf("by ASIN: the true title and missing genres, the narrator kept: %s", wok)
	}
	if bad {
		t.Fatal("an ISBN leading to another title must wait for a parent")
	}
	_, out := admin.do("GET", "/api/admin/audiobookshelf/cleanup", nil, false)
	fixes := out["fixes"].([]any)
	if len(fixes) != 1 || fixes[0].(map[string]any)["item_id"] != "li_bad" || !strings.Contains(fixes[0].(map[string]any)["why"].(string), "Other Book") {
		t.Fatalf("waiting: %v", out)
	}
	res, out := admin.do("POST", "/api/admin/audiobookshelf/fixes/li_bad", map[string]string{"action": "apply"}, true)
	mu.Lock()
	applied := writes["/api/items/li_bad/media"]
	mu.Unlock()
	if res.StatusCode != 200 || len(out["fixes"].([]any)) != 0 || !strings.Contains(applied, `"title":"Other Book"`) {
		t.Fatalf("applied by a parent: %d %v %s", res.StatusCode, out, applied)
	}
	if msg, _ := server.CleanAudiobooks(context.Background(), 50); msg != "" {
		t.Fatalf("nothing again within a month: %q", msg)
	}
}
