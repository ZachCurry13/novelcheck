package events

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// row is one book the way Stuff Your Kindle lists it.
func row(i int, link string) string {
	return fmt.Sprintf(`<article class="book-list-row"><a href="%[2]s" class="cover"><img src="c.jpg" alt="Book Number %[1]d"></a>
		<div><h3><a href="%[2]s">Book Number %[1]d</a></h3><p class="author">By Writer %[1]d</p>
		<p>A story told by nobody in particular.</p>
		<p><a href="%[2]s">Get a copy</a></p><a href="%[2]s" class="btn">Amazon</a></div></article>`, i, link)
}

func TestFetchFollowsLoadMore(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.String())
		switch r.URL.Path {
		case "/days/fall":
			var b strings.Builder
			b.WriteString(`<html><head><title>Fantasy Day — October 16, 2026</title></head><body><h1>Fantasy Day</h1>`)
			for i := 1; i <= 10; i++ {
				b.WriteString(row(i, fmt.Sprintf("https://www.amazon.com/dp/B00000%04d?tag=x", i)))
			}
			b.WriteString(`<button type="button" id="load-more-books-btn" class="btn-load-more" data-offset="10">Load more books</button></body></html>`)
			_, _ = w.Write([]byte(b.String()))
		case "/days/fall/books":
			off, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			var b strings.Builder
			for i := off + 1; i <= min(off+10, 25); i++ {
				b.WriteString(row(i, fmt.Sprintf("https://www.amazon.com/s?k=Book+Number+%d", i)))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"html": b.String(), "hasMore": off+10 < 25, "nextOffset": off + 10})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	old := client
	client = srv.Client() // the test site is on this computer
	defer func() { client = old }()

	page, err := Fetch(context.Background(), srv.URL+"/days/fall")
	if err != nil {
		t.Fatal(err)
	}
	got := Parse(page, "")
	if len(got) != 25 || len(asked) != 3 {
		t.Fatalf("found %d books in %d requests: %v", len(got), len(asked), asked)
	}
	if got[0].ASIN != "B000000001" || got[0].Author != "Writer 1" || got[24].Title != "Book Number 25" || got[24].Author != "Writer 25" {
		t.Fatalf("books: %+v / %+v", got[0], got[24])
	}
	for _, b := range got {
		if strings.Contains(strings.ToLower(b.Title), "copy") {
			t.Fatalf("a button was taken for a book: %+v", b)
		}
	}
	if name, day := Info(page); name != "Fantasy Day" || day != "2026-10-16" {
		t.Fatalf("info: %q %q", name, day)
	}
}

func TestFetchFollowsNextPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := strconv.Atoi(r.URL.Query().Get("page"))
		p = max(p, 1)
		next := ""
		if p < 3 {
			next = fmt.Sprintf(`<link rel="next" href="/list?page=%d">`, p+1)
		}
		fmt.Fprintf(w, `<html><head>%s</head><body><a href="https://www.amazon.com/dp/B0000000%02d">Page Book %d</a> by Someone</body></html>`, next, p, p)
	}))
	defer srv.Close()
	old := client
	client = srv.Client()
	defer func() { client = old }()
	page, err := Fetch(context.Background(), srv.URL+"/list")
	if err != nil {
		t.Fatal(err)
	}
	if got := Parse(page, ""); len(got) != 3 || got[2].Title != "Page Book 3" || got[0].Author != "Someone" {
		t.Fatalf("pages: %+v", got)
	}
}
