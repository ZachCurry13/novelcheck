package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
)

// Searching the libraries finds books NovelCheck doesn't have, and adding one makes the book.
func TestLibbySearchAndAdd(t *testing.T) {
	od := fakeOverDrive(t)
	srv, _ := setupWith(t, func(s *api.Server) { s.Libby.Base = od.URL })
	admin := login(t, srv, "admin", "adminpass1")

	if _, out := admin.do("GET", "/api/libby/search?q=dune", nil, false); out["configured"] != false {
		t.Fatalf("no library yet: %v", out)
	}
	admin.do("PUT", "/api/me/libby", map[string]string{"library": "mylib"}, true)
	_, out := admin.do("GET", "/api/libby/search?q=dune", nil, false)
	books, _ := out["books"].([]any)
	if len(books) != 2 {
		t.Fatalf("both e-books come back: %v", out)
	}
	first, _ := books[1].(map[string]any)
	e, _ := first["ebook"].(map[string]any)
	if first["title"] != "Dune" || e == nil || e["library"] != "My Public Library" || e["holds"].(float64) != 3 {
		t.Fatalf("a row carries the library's answer: %v", first)
	}
	_, short := admin.do("GET", "/api/libby/search?q=d", nil, false)
	if b, _ := short["books"].([]any); len(b) != 0 {
		t.Fatalf("one letter is not a search: %v", short)
	}
	res, made := admin.do("POST", "/api/libby/add", map[string]any{"title": "Dune", "author": "Frank Herbert", "cover": "https://img.example/d.jpg"}, true)
	if res.StatusCode != 200 || made["book_id"] == nil {
		t.Fatalf("add: %d %v", res.StatusCode, made)
	}
	if res, _ := admin.do("POST", "/api/queue", map[string]int64{"book_id": int64(made["book_id"].(float64))}, true); res.StatusCode != 201 {
		t.Fatalf("then onto the Nightstand: %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", "/api/libby/add", map[string]any{"title": " "}, true); res.StatusCode != 400 {
		t.Fatalf("a title is required: %d", res.StatusCode)
	}
}
