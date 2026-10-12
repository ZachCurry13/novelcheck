package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// A Hardcover that knows Dune's edition, starts with it off the person's
// shelves, and records every mutation.
func recordingHardcover(t *testing.T, mutations *[]string) *httptest.Server {
	onShelf, status := false, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var q struct{ Query string }
		_ = json.Unmarshal(raw, &q)
		switch {
		case strings.HasPrefix(q.Query, "mutation"):
			*mutations = append(*mutations, q.Query)
			if strings.Contains(q.Query, "insert_user_book(") {
				onShelf, status = true, 2
			}
			if strings.Contains(q.Query, "update_user_book(") {
				status = 3
			}
			io.WriteString(w, `{"data": {"x": {"__typename": "ok"}}}`)
		case strings.Contains(q.Query, "editions("):
			io.WriteString(w, `{"data": {"editions": [{"id": 5, "book_id": 9, "pages": 500}]}}`)
		case strings.Contains(q.Query, "_in: [2, 3]"): // the pull: nothing yet
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

func TestHardcoverPushesProgress(t *testing.T) {
	var muts []string
	hc := recordingHardcover(t, &muts)
	api.SetHardcoverURL(hc.URL)
	t.Cleanup(func() { api.SetHardcoverURL("") })
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	dune, _ := st.UpsertBook("Dune", "Frank Herbert", "9780441172719", "")
	_ = st.AddCopy(cal, dune, "/c/d.epub", "epub", "1")
	nohc, _ := st.UpsertBook("No ISBN Book", "Nobody", "", "")
	_ = st.AddCopy(cal, nohc, "/c/n.epub", "epub", "2")
	for _, id := range []int64{dune, nohc} {
		admin.do("POST", "/api/queue", map[string]int64{"book_id": id}, true)
	}
	_, items := admin.doList("GET", "/api/queue")
	for _, it := range items {
		admin.do("POST", "/api/queue/"+itoa(int64(it["id"].(float64)))+"/start", nil, true)
	}
	_, _ = st.SaveKOReaderBooks(1, []store.KOReaderBook{{Title: "Dune", Authors: "Frank Herbert", Percent: 0.4, LastOpen: time.Now().Unix() + 60}})

	admin.do("PUT", "/api/me/hardcover", map[string]string{"token": "good-token"}, true)
	if res, _ := admin.do("PUT", "/api/me/hardcover/push", map[string]bool{"on": true}, true); res.StatusCode != 200 {
		t.Fatalf("turn on: %d", res.StatusCode)
	}
	muts = nil
	res, out := admin.do("POST", "/api/me/hardcover/sync", nil, true)
	if res.StatusCode != 200 || out["pushed"] != float64(1) {
		t.Fatalf("sync and push: %d %v", res.StatusCode, out)
	}
	all := strings.Join(muts, "\n")
	for _, want := range []string{"insert_user_book(object: {book_id: 9, edition_id: 5, status_id: 2})", "insert_user_book_read(user_book_id: 3, user_book_read: {progress_pages: 200, edition_id: 5"} {
		if !strings.Contains(all, want) {
			t.Fatalf("missing %q in:\n%s", want, all)
		}
	}
	// Nothing changed: nothing is sent again.
	muts = nil
	admin.do("POST", "/api/me/hardcover/sync", nil, true)
	if len(muts) != 0 {
		t.Fatalf("sent twice: %v", muts)
	}
	// Off by default for others: turning it off stops sending.
	admin.do("PUT", "/api/me/hardcover/push", map[string]bool{"on": false}, true)
	if st.HardcoverPush(1) {
		t.Fatal("still on")
	}
}
