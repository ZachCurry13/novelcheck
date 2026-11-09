package api_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// A failed request is kept with a code (also sent back in X-Error-Id), shows in
// the bug-report link, and "handled" reports are deleted through the link.
func TestServerErrorsAndResolve(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	res, _ := admin.do("POST", "/api/admin/same-books/merge", map[string]int64{"a": 999, "b": 998}, true)
	if res.StatusCode < 400 {
		t.Fatalf("expected a failure, got %d", res.StatusCode)
	}
	id := res.Header.Get("X-Error-Id")
	if res.StatusCode != 404 && !strings.HasPrefix(id, "E-") { // a 404 is "not found", not kept
		t.Fatalf("a failed request carries its code: %d %q", res.StatusCode, id)
	}
	st.LogError(store.ServerError{Code: "E-abcd", Method: "POST", Path: "/kosync/plugin/device", Status: 500, Username: "zach", Detail: "FOREIGN KEY constraint failed"})
	_, out := admin.do("POST", "/api/admin/bug-link", map[string]string{}, true)
	path := out["url"].(string)
	path = path[strings.Index(path, "/bugs/"):]
	get := func() string {
		r, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		return string(b)
	}
	text := get()
	if !strings.Contains(text, "E-abcd") || !strings.Contains(text, "FOREIGN KEY constraint failed") || !strings.Contains(text, "/resolve?ids=") {
		t.Fatalf("the report lists server errors and says how to delete handled ones: %q", text)
	}
	admin.do("POST", "/api/me/bug-report", map[string]string{"text": "first"}, true)
	admin.do("POST", "/api/me/bug-report", map[string]string{"text": "second"}, true)
	logs, _ := st.DeviceLogs()
	if len(logs) != 2 {
		t.Fatalf("two reports: %d", len(logs))
	}
	post := func(q string) int {
		r, err := http.Post(srv.URL+path+"/resolve?"+q, "text/plain", nil)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	if post("ids="+itoa(logs[0].ID)+"&errors=1") != 200 {
		t.Fatal("resolve")
	}
	if left, _ := st.DeviceLogs(); len(left) != 1 || left[0].ID == logs[0].ID {
		t.Fatalf("only the named report is deleted: %v", left)
	}
	if len(st.RecentErrors(10)) != 0 {
		t.Fatal("errors=1 empties the error list")
	}
	if post("all=1") != 200 {
		t.Fatal("all")
	}
	if left, _ := st.DeviceLogs(); len(left) != 0 {
		t.Fatal("all=1 deletes every report")
	}
	// A link that has run out stops working until it is extended.
	_ = st.SetSetting(store.KeyBugLinkExpires, time.Now().UTC().Add(-time.Hour).Format(time.RFC3339))
	if r, _ := http.Get(srv.URL + path); r.StatusCode != 404 {
		t.Fatalf("an expired link is closed: %d", r.StatusCode)
	}
	if post("all=1") != 404 {
		t.Fatal("an expired link cannot delete either")
	}
	_, ext := admin.do("POST", "/api/admin/bug-link?extend=1", map[string]string{}, true)
	if ext["enabled"] != true || ext["url"] != out["url"] {
		t.Fatalf("extending keeps the same link: %v", ext)
	}
	if r, _ := http.Get(srv.URL + path); r.StatusCode != 200 {
		t.Fatalf("extended: %d", r.StatusCode)
	}
	if _, o := admin.do("GET", "/api/admin/bug-link", nil, false); o["seen"] == "" {
		t.Fatalf("opening the link is noted: %v", o)
	}
}

// Slow requests are listed by page in the report, with ids and secrets folded together.
func TestSlowRequestsReport(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	for _, ms := range []string{"1500 ms", "900 ms", "2100 ms"} {
		st.LogError(store.ServerError{Code: "SLOW", Method: "GET", Path: "/api/books/" + itoa(int64(len(ms))) + "/cover-ish", Status: 200, Detail: ms})
	}
	st.LogError(store.ServerError{Code: "SLOW", Method: "GET", Path: "/api/books", Status: 200, Detail: "1200 ms"})
	_, out := admin.do("POST", "/api/admin/bug-link", map[string]string{}, true)
	url := out["url"].(string)
	r, err := http.Get(srv.URL + url[strings.Index(url, "/bugs/"):])
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	text := string(b)
	if !strings.Contains(text, "== Slow requests") || !strings.Contains(text, "GET /api/books/{id}/cover-ish: 3 times, average 1500 ms, worst 2100 ms") {
		t.Fatalf("slow pages are grouped: %q", text[strings.Index(text, "== Slow"):])
	}
	if strings.Contains(text[strings.Index(text, "== Recent server errors"):strings.Index(text, "== Slow")], "SLOW") {
		t.Fatal("slow requests are not listed as errors")
	}
}

// The Library list is kept in memory for a short while, and any change made through the app
// throws the kept copy away.
func TestListCache(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	a, _ := st.UpsertBook("Cached Book", "Someone", "", "")
	_ = st.AddCopy(cat, a, "/c/a.epub", "epub", "1")
	hit := func() string {
		res, _ := admin.do("GET", "/api/books?q=cached", nil, false)
		return res.Header.Get("X-Cache")
	}
	if got := hit(); got != "miss" {
		t.Fatalf("first time is a miss: %q", got)
	}
	if got := hit(); got != "hit" {
		t.Fatalf("second time is a hit: %q", got)
	}
	admin.do("POST", "/api/queue", map[string]int64{"book_id": a}, true)
	if got := hit(); got != "miss" {
		t.Fatalf("a change throws the copy away: %q", got)
	}
	admin.do("POST", "/api/me/bug-report", map[string]string{"text": "x"}, true)
	if got := hit(); got != "hit" {
		t.Fatalf("a report upload does not: %q", got)
	}
}
