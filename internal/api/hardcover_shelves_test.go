package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// shelfHardcover is a Hardcover with one book "Quiet Book" (found only by title search), one Want-to-read book
// on the person's shelves, a reading goal, and a record of what was written.
func shelfHardcover(t *testing.T, mutations *[]string, wanted bool) *httptest.Server {
	onShelf, status := false, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var q struct{ Query string }
		_ = json.Unmarshal(raw, &q)
		switch {
		case strings.HasPrefix(q.Query, "mutation"):
			*mutations = append(*mutations, q.Query)
			if i := strings.Index(q.Query, "status_id: "); i >= 0 {
				status = int(q.Query[i+len("status_id: ")] - '0')
			}
			if strings.Contains(q.Query, "insert_user_book(") {
				onShelf = true
			}
			io.WriteString(w, `{"data": {"x": {"id": 3, "error": null}}}`)
		case strings.Contains(q.Query, "goals("):
			io.WriteString(w, `{"data": {"me": [{"goals": [{"goal": 24, "metric": "book", "progress": 10, "start_date": "2026-01-01", "end_date": "2026-12-31"}]}]}}`)
		case strings.Contains(q.Query, "search("):
			io.WriteString(w, `{"data": {"search": {"ids": ["9"]}}}`)
		case strings.Contains(q.Query, " books(where: {id: {_in"):
			io.WriteString(w, `{"data": {"books": [{"id": 9, "title": "Quiet Book", "contributions": [{"author": {"name": "Quinn Writer"}}], "editions": [{"id": 5, "pages": 300}]}]}}`)
		case strings.Contains(q.Query, "editions("):
			io.WriteString(w, `{"data": {"editions": [{"id": 5, "book_id": 9, "pages": 300}]}}`)
		case strings.Contains(q.Query, "_in: [1, 2, 3, 4]"):
			if wanted {
				io.WriteString(w, `{"data": {"me": [{"user_books": [{"status_id": 1, "updated_at": "2026-01-01T00:00:00Z", "rating": null,
					"book": {"title": "Wanted Book", "pages": 200, "contributions": [{"author": {"name": "Some Author"}}]},
					"edition": {"pages": 200, "isbn_13": "9781111111111"}, "user_book_reads": []}]}]}}`)
				return
			}
			io.WriteString(w, `{"data": {"me": [{"user_books": []}]}}`)
		case strings.Contains(q.Query, "book_id: {_eq: 9}"):
			if !onShelf {
				io.WriteString(w, `{"data": {"me": [{"user_books": []}]}}`)
				return
			}
			io.WriteString(w, `{"data": {"me": [{"user_books": [{"id": 3, "status_id": `+string(rune('0'+status))+`, "user_book_reads": []}]}]}}`)
		default:
			io.WriteString(w, `{"data": {"me": [{"id": 7, "username": "reader7"}]}}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// A book on the Nightstand reaches Hardcover as Want to read (found by title when it has no ISBN), then Reading,
// and when put aside as Paused; goals come back with the reading stats.
func TestHardcoverShelvesAndSearch(t *testing.T) {
	var muts []string
	hc := shelfHardcover(t, &muts, false)
	api.SetHardcoverURL(hc.URL)
	t.Cleanup(func() { api.SetHardcoverURL("") })
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	quiet, _ := st.UpsertBook("Quiet Book", "Quinn Writer", "", "") // no ISBN
	_ = st.AddCopy(cal, quiet, "/c/q.epub", "epub", "1")
	admin.do("POST", "/api/queue", map[string]int64{"book_id": quiet}, true)
	admin.do("PUT", "/api/me/hardcover", map[string]string{"token": "good-token"}, true)
	admin.do("PUT", "/api/me/hardcover/push", map[string]bool{"on": true}, true)
	has := func(want string) bool { return strings.Contains(strings.Join(muts, "\n"), want) }

	muts = nil
	admin.do("POST", "/api/me/hardcover/sync", nil, true)
	if !has("insert_user_book(object: {book_id: 9, edition_id: 5, status_id: 1})") {
		t.Fatalf("Want to read, found by title: %v", muts)
	}
	_, items := admin.doList("GET", "/api/queue")
	itemID := "/api/queue/" + itoa(int64(items[0]["id"].(float64)))
	admin.do("POST", itemID+"/start", nil, true)
	muts = nil
	admin.do("POST", "/api/me/hardcover/sync", nil, true)
	if !has("update_user_book(id: 3, object: {status_id: 2})") {
		t.Fatalf("Reading: %v", muts)
	}
	admin.do("POST", itemID+"/pause", map[string]any{}, true)
	muts = nil
	admin.do("POST", "/api/me/hardcover/sync", nil, true)
	if !has("update_user_book(id: 3, object: {status_id: 4})") {
		t.Fatalf("Paused: %v", muts)
	}
	muts = nil
	admin.do("POST", "/api/me/hardcover/sync", nil, true)
	if len(muts) != 0 {
		t.Fatalf("nothing is sent twice: %v", muts)
	}
	_, stats := admin.do("GET", "/api/me/reading-stats", nil, false)
	goals, _ := stats["hardcover_goals"].([]any)
	if len(goals) != 1 || goals[0].(map[string]any)["goal"] != float64(24) {
		t.Fatalf("goals come back with the stats: %v", stats["hardcover_goals"])
	}
}

// A Want-to-read book at Hardcover that is in the library joins the Nightstand once; taking it off sticks.
func TestHardcoverWantToReadJoinsNightstand(t *testing.T) {
	var muts []string
	hc := shelfHardcover(t, &muts, true)
	api.SetHardcoverURL(hc.URL)
	t.Cleanup(func() { api.SetHardcoverURL("") })
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	wb, _ := st.UpsertBook("Wanted Book", "Some Author", "9781111111111", "")
	_ = st.AddCopy(cal, wb, "/c/w.epub", "epub", "2")
	admin.do("PUT", "/api/me/hardcover", map[string]string{"token": "good-token"}, true)

	_, _ = st.DB.Exec(`DELETE FROM hardcover_books`) // as if the book had only just appeared on the shelf
	admin.do("PUT", "/api/me/hardcover/push", map[string]bool{"on": true}, true)
	admin.do("POST", "/api/me/hardcover/sync", nil, true)
	_, items := admin.doList("GET", "/api/queue")
	if len(items) != 1 || items[0]["title"] != "Wanted Book" {
		t.Fatalf("a new Want to read joins the Nightstand: %v", items)
	}
	admin.do("DELETE", "/api/queue/"+itoa(int64(items[0]["id"].(float64))), nil, true)
	admin.do("POST", "/api/me/hardcover/sync", nil, true)
	if _, items := admin.doList("GET", "/api/queue"); len(items) != 0 {
		t.Fatalf("taking it off the Nightstand sticks: %v", items)
	}
}
