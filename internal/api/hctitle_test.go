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

// Hardcover is the truth for titles: a messy title Hardcover knows tidier is fixed (a Calibre book gets the
// true title as a suggestion to save in Calibre, any other book is renamed), a name a parent typed stays, and
// an ISBN that leads to a different book changes nothing.
func TestHardcoverTrueTitles(t *testing.T) {
	hc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var q struct{ Query string }
		_ = json.Unmarshal(raw, &q)
		switch {
		case strings.Contains(q.Query, "editions(where"):
			io.WriteString(w, `{"data": {"editions": [{"isbn_13": "9780000000099", "book": {"id": 48, "title": "Some Other Book", "contributions": [{"author": {"name": "Ann Author"}}]}}]}}`)
		case strings.Contains(q.Query, `search(query: "Descent into the Void Steve P. Vincent"`):
			io.WriteString(w, `{"data": {"search": {"ids": [44]}}}`)
		case strings.Contains(q.Query, `search(query: "Loving the Lonely Viscount Renee Marski"`):
			io.WriteString(w, `{"data": {"search": {"ids": [45]}}}`)
		case strings.Contains(q.Query, `search(query: "Typed By Parent Pat Writer"`):
			io.WriteString(w, `{"data": {"search": {"ids": [46]}}}`)
		case strings.Contains(q.Query, "search("):
			io.WriteString(w, `{"data": {"search": {"ids": []}}}`)
		case strings.Contains(q.Query, "_in: [44]"):
			io.WriteString(w, `{"data": {"books": [{"id": 44, "title": "Descent into the Void", "contributions": [{"author": {"name": "Steve P. Vincent"}}]}]}}`)
		case strings.Contains(q.Query, "_in: [45]"):
			io.WriteString(w, `{"data": {"books": [{"id": 45, "title": "Loving the Lonely Viscount", "contributions": [{"author": {"name": "Renee Marski"}}]}]}}`)
		case strings.Contains(q.Query, "_in: [46]"):
			io.WriteString(w, `{"data": {"books": [{"id": 46, "title": "Typed By Parent", "contributions": [{"author": {"name": "Pat Writer"}}]}]}}`)
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
		s.Worker.NewEnricher = func() *enrich.Client {
			c := enrich.New("")
			c.OpenLibraryURL, c.GoogleBooksURL = hc.URL, hc.URL
			return c
		}
	})
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("PUT", "/api/me/hardcover", map[string]string{"token": "good-token"}, true)
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	kindle, _ := st.EnsureCatalog("Kindle", "drive")
	calBook, _ := st.UpsertBook("Descent into the Void (An action packed science fiction adventure)", "Steve P. Vincent", "", "")
	_ = st.AddCopy(cal, calBook, "/c/void.epub", "epub", "1")
	plain, _ := st.UpsertBook("Loving the Lonely Viscount: A Chasing Love Novella (Chasing Love Series)", "Renee Marski", "", "")
	_ = st.AddCopy(kindle, plain, "kindle:1", "", "1")
	typed, _ := st.UpsertBook("Typed By Parent (Draft)", "Pat Writer", "", "")
	_ = st.AddCopy(kindle, typed, "kindle:2", "", "2")
	st.MarkName(typed, 2)
	wrongISBN, _ := st.UpsertBook("My Own Book", "Ann Author", "9780000000099", "")
	_ = st.AddCopy(kindle, wrongISBN, "kindle:3", "", "3")

	server.FillRatings(context.Background(), 20)

	if got := st.TitleSuggestion(calBook); got != "Descent into the Void" {
		t.Fatalf("a Calibre book gets the true title as a suggestion: %q", got)
	}
	if b, _ := st.BookByID(plain, nil); b == nil || b.Title != "Loving the Lonely Viscount" {
		t.Fatalf("another book is renamed: %+v", b)
	}
	if b, _ := st.BookByID(typed, nil); b == nil || b.Title != "Typed By Parent (Draft)" {
		t.Fatalf("a name a parent typed stays: %+v", b)
	}
	if b, _ := st.BookByID(wrongISBN, nil); b == nil || b.Title != "My Own Book" {
		t.Fatalf("an ISBN that leads to another book renames nothing: %+v", b)
	}
}
