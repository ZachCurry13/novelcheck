package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Hardcover ratings: found by ISBN and by title, kept on the book, shown in the list, and "Highest rated" puts
// the favorite first and the unrated last.
func TestHardcoverReaderRatings(t *testing.T) {
	hc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var q struct{ Query string }
		_ = json.Unmarshal(raw, &q)
		switch {
		case strings.Contains(q.Query, "editions(where"):
			io.WriteString(w, `{"data": {"editions": [{"isbn_13": "9780000000011", "isbn_10": "", "pages": 300, "book": {"id": 1, "title": "Good Book",
				"rating": 3.9, "ratings_count": 120, "contributions": [{"author": {"name": "Ann Author"}}]}}]}}`)
		case strings.Contains(q.Query, `search(query: "Great Book Ann Author"`):
			io.WriteString(w, `{"data": {"search": {"ids": [2]}}}`)
		case strings.Contains(q.Query, "search("):
			io.WriteString(w, `{"data": {"search": {"ids": []}}}`)
		case strings.Contains(q.Query, "_in: [2]"):
			io.WriteString(w, `{"data": {"books": [{"id": 2, "title": "Great Book", "rating": 4.6, "ratings_count": 900,
				"contributions": [{"author": {"name": "Ann Author"}}], "editions": [{"pages": 300, "isbn_13": "9780000000028", "isbn_10": ""}]}]}}`)
		default:
			io.WriteString(w, `{"data": {"me": [{"id": 7, "username": "reader7"}]}}`)
		}
	}))
	defer hc.Close()
	api.SetHardcoverURL(hc.URL)
	t.Cleanup(func() { api.SetHardcoverURL("") })
	var server *api.Server
	srv, st := setupWith(t, func(s *api.Server) {
		server = s
		s.Worker.NewEnricher = func() *enrich.Client { // Open Library, asked for the book Hardcover doesn't know: nothing
			c := enrich.New("")
			c.OpenLibraryURL, c.GoogleBooksURL = hc.URL, hc.URL
			return c
		}
	})
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	for i, b := range [][2]string{{"Good Book", "9780000000011"}, {"Great Book", ""}, {"Unknown Book", ""}} {
		id, _ := st.UpsertBook(b[0], "Ann Author", b[1], "")
		_ = st.AddCopy(cal, id, "/c/"+b[0]+".epub", "epub", itoa(int64(i+1)))
	}
	if n := server.FillRatings(context.Background(), 10); n != 0 {
		t.Fatalf("nobody connected Hardcover yet: %d", n)
	}
	admin.do("PUT", "/api/me/hardcover", map[string]string{"token": "good-token"}, true)
	if n := server.FillRatings(context.Background(), 10); n != 2 {
		t.Fatalf("two books rated: %d", n)
	}
	_, out := admin.do("GET", "/api/books?sort=rating", nil, false)
	var got []string
	for _, b := range out["books"].([]any) {
		m := b.(map[string]any)
		got = append(got, m["title"].(string))
		if m["title"] == "Great Book" && (m["hc_rating"] != 4.6 || m["hc_ratings"] != 900.0) {
			t.Fatalf("kept on the book: %v", m)
		}
	}
	if strings.Join(got, ",") != "Great Book,Good Book,Unknown Book" {
		t.Fatalf("highest rated first, unrated last: %v", got)
	}
	if n := server.FillRatings(context.Background(), 10); n != 0 {
		t.Fatalf("looked again only after a month: %d", n)
	}
	// The book with no ISBN got Hardcover's; the one nobody knows still has none.
	var isbns []string
	_ = st.DB.Select(&isbns, `SELECT isbn FROM books WHERE title IN ('Great Book', 'Unknown Book') ORDER BY title`)
	if strings.Join(isbns, ",") != "9780000000028," {
		t.Fatalf("ISBNs: %q", isbns)
	}
}

// An ISBN found for a Calibre book is written into Calibre's identifiers, keeping the ones it has.
func TestFoundISBNGoesToCalibre(t *testing.T) {
	hc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var q struct{ Query string }
		_ = json.Unmarshal(raw, &q)
		switch {
		case strings.Contains(q.Query, "search("):
			io.WriteString(w, `{"data": {"search": {"ids": [3]}}}`)
		case strings.Contains(q.Query, "_in: [3]"):
			io.WriteString(w, `{"data": {"books": [{"id": 3, "title": "Cinder", "rating": 4.1, "ratings_count": 50,
				"contributions": [{"author": {"name": "Marissa Meyer"}}], "editions": [{"pages": 400, "isbn_13": "9780312641894", "isbn_10": ""}]}]}}`)
		default:
			io.WriteString(w, `{"data": {"me": [{"id": 7, "username": "reader7"}]}}`)
		}
	}))
	defer hc.Close()
	api.SetHardcoverURL(hc.URL)
	t.Cleanup(func() { api.SetHardcoverURL("") })
	cal := &fakeCalibreDetails{book: map[string]any{"title": "Cinder", "authors": "Marissa Meyer", "series": "", "series_index": 1.0,
		"tags": []string{}, "comments": "", "identifiers": map[string]string{"goodreads": "123"}, "cover": ""}}
	cs := cal.serve(t)
	var server *api.Server
	srv, st := setupWith(t, func(s *api.Server) { server = s })
	admin := login(t, srv, "admin", "adminpass1")
	catalog, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	id, _ := st.UpsertBook("Cinder", "Marissa Meyer", "", "")
	_ = st.AddCopy(catalog, id, "/c/cinder.epub", "epub", "5")
	admin.do("PUT", "/api/me/hardcover", map[string]string{"token": "good-token"}, true)
	admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": cs.URL}, true)
	server.FillRatings(context.Background(), 10)
	ids, _ := cal.book["identifiers"].(map[string]any)
	if ids["isbn"] != "9780312641894" || ids["goodreads"] != "123" {
		t.Fatalf("Calibre's identifiers: %v", cal.book["identifiers"])
	}
}
