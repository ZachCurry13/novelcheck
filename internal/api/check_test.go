package api_test

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// fakeBooks stands in for Open Library, Google Books and a vision-capable AI.
func fakeBooks(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/search.json":
			q := r.URL.Query()
			doc := map[string]any{}
			switch {
			case strings.Contains(q.Get("q"), "hobbit"):
				doc = map[string]any{"title": "The Hobbit", "author_name": []string{"J.R.R. Tolkien"}, "key": "/works/OL1W"}
			case q.Get("title") == "FOURTH WING":
				doc = map[string]any{"title": "Fourth Wing", "author_name": []string{"Rebecca Yarros"}, "isbn": []string{"1649374046", "9781649374042"}, "key": "/works/OL2W"}
			}
			docs := []any{}
			if len(doc) > 0 {
				docs = append(docs, doc)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"docs": docs})
		case strings.HasPrefix(r.URL.Path, "/works/"):
			_ = json.NewEncoder(w).Encode(map[string]string{"description": "A war college for dragon riders, with a slow-burn romance between two rivals that turns explicit."})
		case r.URL.Path == "/chat/completions":
			raw, _ := io.ReadAll(r.Body)
			answer := `{"spice_level": 4, "spice_reason": "Several explicit scenes", "summary_verdict": "Explicit romance."}`
			if strings.Contains(string(raw), `"image_url"`) {
				answer = `{"title": "FOURTH WING", "author": "REBECCA YARROS", "isbn": ""}`
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": answer}}},
				"usage":   map[string]int{"prompt_tokens": 100, "completion_tokens": 20},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCheckABook(t *testing.T) {
	fake := fakeBooks(t)
	srv, st := setupWith(t, func(s *api.Server) {
		s.Worker.NewEnricher = func() *enrich.Client {
			c := enrich.New("")
			c.OpenLibraryURL, c.GoogleBooksURL = fake.URL, fake.URL
			return c
		}
	})
	_ = st.SetSetting(store.KeyLLMBaseURL, fake.URL)
	_ = st.SetSetting(store.KeyLLMModel, "vision-model")
	admin := login(t, srv, "admin", "adminpass1")

	// Already in the library: its rating comes back straight away.
	cat, _ := st.EnsureCatalog("Calibre Main", "calibre")
	hobbit, _ := st.UpsertBook("The Hobbit", "J. R. R. Tolkien", "", "")
	_ = st.AddCopy(cat, hobbit, "/calibre/hobbit.epub", "epub", "7")
	zero := 0
	_ = st.SaveAnalysis(hobbit, store.Analysis{SpiceLevel: &zero, Model: "gpt"})
	res, out := admin.do("POST", "/api/check", map[string]string{"query": "the hobbit tolkien"}, true)
	if res.StatusCode != 200 || out["rating"] != false || out["in_library"] != true || out["book"].(map[string]any)["id"].(float64) != float64(hobbit) {
		t.Fatalf("library match: %d %v", res.StatusCode, out)
	}

	// A photo of a book we don't have: read, looked up, saved and rated now.
	photo := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString([]byte("fake jpeg bytes"))
	res, out = admin.do("POST", "/api/check", map[string]string{"image": photo}, true)
	if res.StatusCode != 200 || out["source"] != "photo" || out["rating"] != true || out["in_library"] != false {
		t.Fatalf("photo check: %d %v", res.StatusCode, out)
	}
	b := out["book"].(map[string]any)
	if b["title"] != "Fourth Wing" || b["author"] != "Rebecca Yarros" || b["isbn"] != "9781649374042" {
		t.Fatalf("catalogued spelling expected: %v", b)
	}
	id := int64(b["id"].(float64))
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		got, _ := st.BookByID(id, nil)
		if got.Status == "analyzed" && got.SpiceLevel != nil && *got.SpiceLevel == 4 && got.SpiceReason == "Several explicit scenes" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("not rated: %+v", got)
		}
	}
	// Checking it again is instant: no second rating.
	if _, out = admin.do("POST", "/api/check", map[string]string{"query": "Fourth Wing"}, true); out["rating"] != false {
		t.Fatalf("second check should reuse the rating: %v", out)
	}
	copies, _ := st.BookCopies(id)
	if len(copies) != 1 || copies[0].CatalogName != store.LookedUpCatalog {
		t.Fatalf("saved to Looked up: %+v", copies)
	}
	// A looked-up book isn't ours: there's nothing to delete.
	if res, _ := admin.do("POST", "/api/books/"+itoa(id)+"/delete-request", map[string]string{"reason": "x"}, true); res.StatusCode != 400 {
		t.Fatalf("delete request for an unowned book: %d", res.StatusCode)
	}

	for _, bad := range []map[string]string{{}, {"image": "data:text/html;base64,PGI+"}, {"query": "978-0-00-000000-0"}} {
		if res, _ := admin.do("POST", "/api/check", bad, true); res.StatusCode == 200 {
			t.Fatalf("%v should be refused", bad)
		}
	}
	admin.do("POST", "/api/admin/users", map[string]string{"username": "kid", "password": "kidpass12"}, true)
	if res, _ := login(t, srv, "kid", "kidpass12").do("POST", "/api/check", map[string]string{"query": "x"}, true); res.StatusCode != 403 {
		t.Fatalf("Check a book is for parents: %d", res.StatusCode)
	}
}
