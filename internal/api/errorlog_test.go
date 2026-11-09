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
