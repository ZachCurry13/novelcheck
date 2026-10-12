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

// A fake Hardcover: `me` as a list, like the real one; "bad" is refused.
func fakeHardcover(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var q struct{ Query string }
		_ = json.Unmarshal(raw, &q)
		now := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
		if strings.Contains(q.Query, "user_books") {
			io.WriteString(w, `{"data": {"me": [{"user_books": [
			 {"status_id": 2, "updated_at": "`+now+`", "book": {"title": "Dune", "pages": 400, "contributions": [{"author": {"name": "Frank Herbert"}}]},
			  "edition": {"pages": 500, "isbn_10": "", "isbn_13": "9780441172719"}, "user_book_reads": [{"progress_pages": 125}]},
			 {"status_id": 3, "updated_at": "`+now+`", "book": {"title": "Mort", "pages": 300, "contributions": [{"author": {"name": "Terry Pratchett"}}]},
			  "edition": null, "user_book_reads": []},
			 {"status_id": 3, "updated_at": "`+now+`", "book": {"title": "Not In The Library", "pages": 10, "contributions": []}, "edition": null, "user_book_reads": []}]}]}}`)
			return
		}
		io.WriteString(w, `{"data": {"me": [{"id": 7, "username": "reader7"}]}}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestHardcoverSync(t *testing.T) {
	hc := fakeHardcover(t)
	api.SetHardcoverURL(hc.URL)
	t.Cleanup(func() { api.SetHardcoverURL("") })
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	dune, _ := st.UpsertBook("Dune", "Frank Herbert", "9780441172719", "")
	mort, _ := st.UpsertBook("Mort", "Terry Pratchett", "", "")
	for i, id := range []int64{dune, mort} {
		_ = st.AddCopy(cal, id, "/c/x.epub", "epub", string(rune('1'+i)))
		admin.do("POST", "/api/queue", map[string]int64{"book_id": id}, true)
	}
	if res, _ := admin.do("PUT", "/api/me/hardcover", map[string]string{"token": "wrong"}, true); res.StatusCode != 502 {
		t.Fatalf("a bad token: %d", res.StatusCode)
	}
	res, out := admin.do("PUT", "/api/me/hardcover", map[string]string{"token": "Bearer good-token"}, true)
	if res.StatusCode != 200 || out["username"] != "reader7" || out["books"].(float64) != 3 || out["matched"].(float64) != 2 {
		t.Fatalf("connect: %d %v", res.StatusCode, out)
	}
	if _, me := admin.do("GET", "/api/me", nil, false); strings.Contains(toJSON(me), "good-token") {
		t.Fatal("the token leaked to the browser")
	}
	// Dune: reading at 125 of 500 pages; Mort: finished. Both were in Up Next.
	_, items := admin.doList("GET", "/api/queue?all=1")
	got := map[string]string{}
	for _, it := range items {
		got[it["title"].(string)] = it["status"].(string)
		if it["title"] == "Dune" {
			if p, _ := it["progress"].(map[string]any); p == nil || p["percent"].(float64) != 0.25 || p["device"] != "Hardcover" {
				t.Fatalf("Dune progress: %v", it["progress"])
			}
		}
	}
	if got["Dune"] != "reading" || got["Mort"] != "finished" {
		t.Fatalf("up next: %v", got)
	}
	if res, _ := admin.do("POST", "/api/me/hardcover/sync", nil, true); res.StatusCode != 200 {
		t.Fatalf("sync now: %d", res.StatusCode)
	}
	if _, out := admin.do("PUT", "/api/me/hardcover", map[string]string{"token": ""}, true); out["connected"] != false || out["books"].(float64) != 0 {
		t.Fatalf("disconnect: %v", out)
	}
}

func toJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
