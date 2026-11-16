package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// A Calibre book with a badly typed title ("Unnatural.Exprosure") gets its true title from the author's books
// on Open Library: offered in Review covers, and once the name finder has kept it, listed under Tidy titles.
func TestTrueTitleForACalibreBook(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		docs := []any{}
		if r.URL.Path == "/search.json" && strings.Contains(r.URL.Query().Get("author"), "Cornwell") {
			for _, t := range []string{"Unnatural Exposure", "Postmortem", "The Last Precinct"} {
				docs = append(docs, map[string]any{"title": t, "author_name": []string{"Patricia Cornwell"}})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"docs": docs, "items": []any{}})
	}))
	defer fake.Close()
	srv, st := setupWith(t, func(s *api.Server) {
		s.Worker.NewEnricher = func() *enrich.Client {
			c := enrich.New("")
			c.OpenLibraryURL, c.GoogleBooksURL = fake.URL, fake.URL
			return c
		}
		s.Covers.SearchURL, s.Covers.GoogleURL = fake.URL, fake.URL
	})
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	id, _ := st.UpsertBook("Unnatural.Exprosure", "Patricia Cornwell", "", "")
	_ = st.AddCopy(cal, id, "/c/u.epub", "epub", "41")

	res, out := admin.do("GET", sprintf("/api/admin/covers/suggest?id=%d", id), nil, false)
	if res.StatusCode != 200 || out["clean_title"] != "Unnatural Exposure" || out["in_calibre"] != true {
		t.Fatalf("suggest: %d %v", res.StatusCode, out)
	}
	st.SuggestTitle(id, "Unnatural Exposure")
	fixes, _ := st.TitleFixes()
	if len(fixes) != 1 || fixes[0].Title != "Unnatural Exposure" || fixes[0].CalibreTitle != "Unnatural.Exprosure" {
		t.Fatalf("tidy titles list: %+v", fixes)
	}
	if st.CountTitleFixes() != 1 {
		t.Fatal("not counted under Tidy titles")
	}
}
