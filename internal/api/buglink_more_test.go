package api_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The bug link also shows what the family reported, and deletes single server errors, family reports and
// the debug recording once they are handled.
func TestBugLinkProblemsAndSingleErrors(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	st.LogError(store.ServerError{Code: "E-aaaa", Method: "POST", Path: "/api/x", Status: 500, Detail: "first cause"})
	st.LogError(store.ServerError{Code: "E-bbbb", Method: "POST", Path: "/api/y", Status: 500, Detail: "second cause"})
	if res, _ := admin.do("POST", "/api/problems", map[string]string{"text": "The cover of Cinder is wrong", "page": "#/library"}, true); res.StatusCode >= 300 {
		t.Fatalf("report a problem: %d", res.StatusCode)
	}
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
	if !strings.Contains(text, "The cover of Cinder is wrong") || !strings.Contains(text, "--- P") || !strings.Contains(text, "problems=") {
		t.Fatalf("the link lists what the family reported and how to mark it done: %q", text[max(0, len(text)-900):])
	}
	open, _ := st.ProblemReports()
	r, err := http.Post(srv.URL+path+"/resolve?errors=E-aaaa&problems=P"+itoa(open[0].ID)+"&recording=1", "text/plain", nil)
	if err != nil || r.StatusCode != 200 {
		t.Fatalf("resolve: %v %v", r, err)
	}
	r.Body.Close()
	left := st.RecentErrors(10)
	if len(left) != 1 || left[0].Code != "E-bbbb" {
		t.Fatalf("only the named error is deleted: %+v", left)
	}
	if open, _ := st.ProblemReports(); len(open) != 0 {
		t.Fatalf("the family report is done: %+v", open)
	}
	if text := get(); strings.Contains(text, "The cover of Cinder is wrong") || strings.Contains(text, "first cause") {
		t.Fatal("handled things are gone from the link")
	}
}
