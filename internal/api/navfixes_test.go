package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// A kid's AI features stay off until a parent allows them; everyone can
// pick the page NovelCheck opens on (kids not Check a book).
func TestKidsAIAndStartPage(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]string{"username": "kid", "password": "kidpass12"}, true)
	kid, _ := st.UserByName("kid")
	child := login(t, srv, "kid", "kidpass12")
	id, _ := st.UpsertBook("A Book", "Author", "", "")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	_ = st.AddCopy(cal, id, "/c/a.epub", "epub", "1")
	zero := 0
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &zero, ContentSource: store.SourceAI})

	if res, _ := child.do("POST", "/api/books/"+itoa(id)+"/deep-scan", nil, true); res.StatusCode != http.StatusForbidden {
		t.Fatalf("kid asking for a Deep Scan without AI: %d", res.StatusCode)
	}
	_, out := child.do("GET", "/api/books/"+itoa(id)+"/deep-scan", nil, false)
	if out["available"] != false || out["why"] == nil {
		t.Fatalf("kid's Deep Scan info: %v", out)
	}
	kid, _ = st.UserByID(kid.ID)
	kid.AIFeatures = true
	if res, _ := admin.do("PUT", "/api/admin/users/"+itoa(kid.ID), kid, true); res.StatusCode != 200 {
		t.Fatalf("allow AI: %d", res.StatusCode)
	}
	if k, _ := st.UserByID(kid.ID); !k.AIFeatures || !k.MayUseAI() {
		t.Fatalf("not allowed: %+v", k)
	}

	if res, _ := child.do("PUT", "/api/me/start-page", map[string]string{"page": "check"}, true); res.StatusCode != 400 {
		t.Fatalf("kid start page check: %d", res.StatusCode)
	}
	if res, out := child.do("PUT", "/api/me/start-page", map[string]string{"page": "collections"}, true); res.StatusCode != 200 || out["start_page"] != "collections" {
		t.Fatalf("kid start page: %d %v", res.StatusCode, out)
	}
	if _, me := child.do("GET", "/api/me", nil, false); me["start_page"] != "collections" {
		t.Fatalf("me: %v", me["start_page"])
	}
}

// A typed title Open Library can't match for certain comes back as choices;
// the chosen one is then used as it is.
func TestTypedTitleChoices(t *testing.T) {
	ol := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		docs := []map[string]any{
			{"title": "The Adventures of Tom Sawyer", "author_name": []string{"Mark Twain"}},
			{"title": "Inkheart", "author_name": []string{"Cornelia Funke"}, "isbn": []string{"9780439531641"}},
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"docs": docs})
	}))
	defer ol.Close()
	srv, st := setupWith(t, func(s *api.Server) {
		s.Worker.NewEnricher = func() *enrich.Client {
			c := enrich.New("")
			c.OpenLibraryURL, c.GoogleBooksURL = ol.URL, ol.URL
			return c
		}
	})
	admin := login(t, srv, "admin", "adminpass1")
	_, out := admin.do("POST", "/api/shelves", map[string]string{"name": "Shelf"}, true)
	shelf := itoa(int64(out["id"].(float64)))
	res, out := admin.do("POST", "/api/shelves/"+shelf+"/books", map[string]string{"query": "ink world by funke"}, true)
	if choices, _ := out["choices"].([]any); res.StatusCode != 200 || len(choices) != 2 || out["book"] != nil {
		t.Fatalf("choices: %d %v", res.StatusCode, out)
	}
	res, out = admin.do("POST", "/api/shelves/"+shelf+"/books", map[string]any{"query": "ink world by funke",
		"pick": map[string]string{"title": "Inkheart", "author": "Cornelia Funke", "isbn": "9780439531641"}}, true)
	b, _ := out["book"].(map[string]any)
	if res.StatusCode != 200 || b["title"] != "Inkheart" || b["author"] != "Cornelia Funke" {
		t.Fatalf("picked: %d %v", res.StatusCode, out)
	}
	if id := st.MatchBook("The Adventures of Tom Sawyer", "Mark Twain"); id != 0 {
		t.Fatal("the wrong book was added")
	}
}
