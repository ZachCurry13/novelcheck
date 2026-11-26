package freebooks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFindAndFetch(t *testing.T) {
	hits := 0
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/books/"):
			hits++
			_, _ = w.Write([]byte(`{"results":[
				{"id":1,"title":"Frankenstein Unbound","authors":[{"name":"Aldiss, Brian"}],"formats":{"application/epub+zip":"` + srv.URL + `/f1.epub"}},
				{"id":84,"title":"Frankenstein; Or, The Modern Prometheus","authors":[{"name":"Shelley, Mary Wollstonecraft"}],"formats":{"application/epub+zip":"` + srv.URL + `/f84.epub"}}]}`))
		case r.URL.Path == "/f84.epub":
			_, _ = w.Write([]byte("PK\x03\x04 pretend epub"))
		default:
			_, _ = w.Write([]byte("<html>not a book"))
		}
	}))
	defer srv.Close()
	c := New()
	c.Base = srv.URL
	m, ok := c.Find(context.Background(), "Frankenstein", "Mary Shelley")
	if !ok || m.ID != 84 || m.Author != "Shelley, Mary Wollstonecraft" {
		t.Fatalf("find: %+v %v", m, ok)
	}
	if _, ok := c.Find(context.Background(), "Frankenstein", "Mary Shelley"); !ok || hits != 1 {
		t.Fatalf("an answer is kept: hits %d", hits)
	}
	if _, ok := c.Find(context.Background(), "Frankenstein", "Stephen King"); ok {
		t.Fatal("the author has to agree")
	}
	if _, ok := c.Find(context.Background(), "Fr", ""); ok {
		t.Fatal("a very short title is not searched")
	}
	data, err := c.Fetch(context.Background(), m)
	if err != nil || !strings.HasPrefix(string(data), "PK") {
		t.Fatalf("fetch: %v", err)
	}
	if _, err := c.Fetch(context.Background(), Match{EPUB: srv.URL + "/other"}); err == nil {
		t.Fatal("not an e-book is refused")
	}
}
