package hardcover

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A refused write (200 with an "error" text) is an error, calls say what they are, and they are spaced out.
func TestMutationErrorsAndPacing(t *testing.T) {
	var agents []string
	var times []time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agents = append(agents, r.Header.Get("User-Agent"))
		times = append(times, time.Now())
		if len(times) == 1 {
			w.Write([]byte(`{"data": {"update_user_book": {"id": 3, "error": "Book not found"}}}`))
			return
		}
		w.Write([]byte(`{"data": {"update_user_book": {"id": 3, "error": null}}}`))
	}))
	defer srv.Close()
	old := MinGap
	MinGap = 60 * time.Millisecond
	defer func() { MinGap = old }()
	c := &Client{Token: "t", URL: srv.URL}

	err := c.SetRating(context.Background(), 3, 4.25)
	if err == nil || !strings.Contains(err.Error(), "Book not found") {
		t.Fatalf("a refused write is an error: %v", err)
	}
	if err := c.SetRating(context.Background(), 3, 4.25); err != nil {
		t.Fatalf("a good write: %v", err)
	}
	if !strings.Contains(agents[0], "NovelCheck") {
		t.Fatalf("says what it is: %q", agents[0])
	}
	if gap := times[1].Sub(times[0]); gap < 55*time.Millisecond {
		t.Fatalf("calls are spaced: %v", gap)
	}
}
