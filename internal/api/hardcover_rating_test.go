package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
)

// A Hardcover with Dune rated 4.5 on the person's shelves; mutations are recorded.
func ratedHardcover(t *testing.T, mutations *[]string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var q struct{ Query string }
		_ = json.Unmarshal(raw, &q)
		switch {
		case strings.HasPrefix(q.Query, "mutation"):
			*mutations = append(*mutations, q.Query)
			io.WriteString(w, `{"data": {"x": {"__typename": "ok"}}}`)
		case strings.Contains(q.Query, "editions("):
			io.WriteString(w, `{"data": {"editions": [{"id": 5, "book_id": 9, "pages": 500}]}}`)
		case strings.Contains(q.Query, "_in: [1, 2, 3, 4]"):
			io.WriteString(w, `{"data": {"me": [{"user_books": [{"status_id": 3, "updated_at": "2026-01-01T00:00:00Z", "rating": 4.5,
				"book": {"title": "Dune", "pages": 500, "contributions": [{"author": {"name": "Frank Herbert"}}]},
				"edition": {"pages": 500, "isbn_13": "9780441172719"}, "user_book_reads": [{"progress_pages": 500, "finished_at": "2026-03-09"}]}]}]}}`)
		case strings.Contains(q.Query, "book_id: {_eq: 9}"):
			io.WriteString(w, `{"data": {"me": [{"user_books": [{"id": 3, "status_id": 3, "user_book_reads": []}]}]}}`)
		default:
			io.WriteString(w, `{"data": {"me": [{"id": 7, "username": "reader7"}]}}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Ratings come from Hardcover when you have none here (never replacing yours), and the ones you give here
// go to Hardcover for books on your shelves.
func TestHardcoverRatings(t *testing.T) {
	var muts []string
	hc := ratedHardcover(t, &muts)
	api.SetHardcoverURL(hc.URL)
	t.Cleanup(func() { api.SetHardcoverURL("") })
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	dune, _ := st.UpsertBook("Dune", "Frank Herbert", "9780441172719", "")
	mort, _ := st.UpsertBook("Mort", "Terry Pratchett", "9780575099654", "")

	admin.do("PUT", "/api/me/hardcover", map[string]string{"token": "good-token"}, true)
	admin.do("PUT", "/api/me/hardcover/push", map[string]bool{"on": true}, true)
	admin.do("POST", "/api/me/hardcover/sync", nil, true)
	if got := st.RatingOf(1, dune); got != 4.5 {
		t.Fatalf("a Hardcover rating fills the gap here: %v", got)
	}
	if len(muts) != 0 {
		t.Fatalf("a rating that came from Hardcover is not sent back: %v", muts)
	}
	_, stats := admin.do("GET", "/api/me/reading-stats", nil, false)
	read, _ := stats["hardcover_read"].([]any)
	if len(read) != 1 || read[0].(map[string]any)["finished"] != "2026-03-09" || read[0].(map[string]any)["pages"] != float64(500) {
		t.Fatalf("the finish day comes back for Wrapped: %v", stats["hardcover_read"])
	}
	admin.do("PUT", "/api/books/"+itoa(dune)+"/rating", map[string]float64{"stars": 5}, true)
	admin.do("POST", "/api/me/hardcover/sync", nil, true)
	if got := st.RatingOf(1, dune); got != 5 {
		t.Fatalf("yours here is never replaced by a sync: %v", got)
	}
	if all := strings.Join(muts, "\n"); !strings.Contains(all, "update_user_book(id: 3, object: {rating: 5})") {
		t.Fatalf("the new rating is sent: %v", muts)
	}
	muts = nil
	admin.do("PUT", "/api/books/"+itoa(mort)+"/rating", map[string]float64{"stars": 3.75}, true)
	admin.do("POST", "/api/me/hardcover/sync", nil, true)
	if all := strings.Join(muts, "\n"); !strings.Contains(all, "rating: 4}") {
		t.Fatalf("quarter stars go to Hardcover as the nearest half star: %v", muts)
	}
	muts = nil
	admin.do("POST", "/api/me/hardcover/sync", nil, true)
	if len(muts) != 0 {
		t.Fatalf("sent twice: %v", muts)
	}
}
