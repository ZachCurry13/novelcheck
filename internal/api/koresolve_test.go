package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// KOReader books that match nothing in the library are looked up in Open
// Library, which gives them a cover and a real title (without joining the library).
func TestKOReaderBooksFindOpenLibrary(t *testing.T) {
	var asked []string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		asked = append(asked, q.Get("title")+q.Get("q"))
		docs := []any{}
		if q.Get("title") == "Divergent" {
			docs = append(docs, map[string]any{"title": "Divergent", "author_name": []string{"Veronica Roth"}, "isbn": []string{"9780062024039"}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"docs": docs})
	}))
	t.Cleanup(fake.Close)
	srv, st := setupWith(t, func(s *api.Server) {
		s.Worker.NewEnricher = func() *enrich.Client {
			c := enrich.New("")
			c.OpenLibraryURL, c.GoogleBooksURL = fake.URL, fake.URL
			return c
		}
	})
	admin := login(t, srv, "admin", "adminpass1")
	cat, _ := st.EnsureCatalog("Calibre Main", "calibre")
	hostile, _ := st.UpsertBook("The Hostile Hospital", "Lemony Snicket", "", "")
	_ = st.AddCopy(cat, hostile, "/c/h.epub", "epub", "1")
	_, err := st.SaveKOReaderBooks(1, []store.KOReaderBook{
		{Title: "Hostile Hospital", Authors: "Snicket, Lemony", Percent: 0.5},            // in the library under "The ..."
		{Title: "Divergent", Authors: "Veronica Roth", Percent: 0.01},                    // not in the library: Open Library knows it
		{Title: "KOReader Quickstart Guide", Authors: "N/A", Percent: 1},                 // not a book to look up
		{Title: "thumbnail_1214814143898814077", Authors: "N/A", Percent: 1},             // a file, not a book
		{Title: "Something Nobody Has Heard Of", Authors: "Nobody At All", Percent: 0.3}, // Open Library has nothing
	})
	if err != nil {
		t.Fatal(err)
	}

	var books []any
	for i := 0; i < 50; i++ { // the lookup runs in the background
		_, out := admin.do("GET", "/api/me/koreader-books", nil, false)
		books, _ = out["books"].([]any)
		if len(books) == 5 && booksBy(books, "Divergent")["book_id"] != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if h := booksBy(books, "Hostile Hospital"); h["book_id"] != float64(hostile) || h["owned"] != true {
		t.Fatalf("Hostile Hospital should match the library: %v", h)
	}
	if d := booksBy(books, "Divergent"); d["book_id"] == nil || d["owned"] != false {
		t.Fatalf("Divergent should be looked up, not owned: %v", d)
	}
	for _, title := range []string{"KOReader Quickstart Guide", "thumbnail_1214814143898814077", "Something Nobody Has Heard Of"} {
		if b := booksBy(books, title); b["book_id"] != nil {
			t.Fatalf("%s should stay unmatched: %v", title, b)
		}
	}
	for _, a := range asked {
		if a == "KOReader Quickstart Guide" || a == "thumbnail_1214814143898814077" {
			t.Fatalf("looked up something that is not a book: %v", asked)
		}
	}
}

func booksBy(books []any, title string) map[string]any {
	for _, b := range books {
		if m := b.(map[string]any); m["title"] == title {
			return m
		}
	}
	return map[string]any{}
}
