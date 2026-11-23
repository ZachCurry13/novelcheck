package enrich

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// After Google says its daily limit is used up, the blurb lookups stop asking it until the limit starts again.
func TestGoogleRestsAfterLimit(t *testing.T) {
	asked := 0
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/volumes") {
			asked++
			http.Error(w, "quota", http.StatusTooManyRequests)
			return
		}
		http.NotFound(w, r)
	}))
	defer fake.Close()
	defer func() { googleRest.mu.Lock(); googleRest.until = time.Time{}; googleRest.mu.Unlock() }()
	c := &Client{HTTP: fake.Client(), OpenLibraryURL: fake.URL, GoogleBooksURL: fake.URL + "/books/v1"}
	c.Blurb(context.Background(), "A Book", "Someone", "", "")
	c.Blurb(context.Background(), "Another Book", "Someone", "", "")
	if asked != 1 || !googleResting() {
		t.Fatalf("Google asked %d times; want once, then a rest", asked)
	}
}
