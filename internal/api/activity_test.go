package api_test

import (
	"strings"
	"testing"
	"time"
)

// Debug recording: off by default, an admin turns it on for a while, browsers send what happened, the server
// notes its own calls, the timeline shows in the admin view and in the server report, and it can be stopped
// and deleted.
func TestDebugRecording(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")

	if _, out := admin.do("GET", "/api/activity/status", nil, false); out["on"] != false {
		t.Fatalf("off by default: %v", out)
	}
	// Sent while off: ignored.
	admin.do("POST", "/api/activity/events", map[string]any{"events": []map[string]any{{"t": time.Now().UnixMilli(), "kind": "tap", "detail": "button ignored"}}}, true)
	if n := len(st.ActivityLog(10)); n != 0 {
		t.Fatalf("nothing is kept while off: %d", n)
	}

	_, out := admin.do("PUT", "/api/admin/activity", map[string]int{"minutes": 500}, true)
	if out["on"] != true || out["seconds_left"].(float64) > 3600 {
		t.Fatalf("at most an hour: %v", out)
	}
	if _, out := admin.do("GET", "/api/activity/status", nil, false); out["on"] != true {
		t.Fatalf("everyone sees it is on: %v", out)
	}
	admin.do("POST", "/api/activity/events", map[string]any{"events": []map[string]any{
		{"t": time.Now().UnixMilli(), "kind": "tap", "detail": "button Tidy selected in Calibre"},
		{"t": time.Now().UnixMilli(), "kind": "weird", "detail": "kept as a note"},
	}}, true)
	admin.do("GET", "/api/books?q=dune", nil, false) // the server notes its own calls, with the query

	txt := st.ActivityLog(50)
	var all []string
	for _, e := range txt {
		all = append(all, e.Source+" "+e.Kind+" "+e.Detail)
	}
	joined := strings.Join(all, "\n")
	for _, want := range []string{"browser tap button Tidy selected in Calibre", "browser note kept as a note", "server net GET /api/books?q=dune -> 200"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "/api/activity/") {
		t.Fatalf("its own polling is not recorded:\n%s", joined)
	}

	res, rep := admin.do("GET", "/api/admin/activity", nil, false)
	if res.StatusCode != 200 || len(rep["events"].([]any)) < 3 {
		t.Fatalf("admin timeline: %d %v", res.StatusCode, rep)
	}
	if res, _ := admin.do("GET", "/api/admin/diagnostics", nil, false); res.StatusCode != 200 {
		t.Fatalf("diagnostics: %d", res.StatusCode)
	}

	admin.do("PUT", "/api/admin/activity", map[string]int{"minutes": 0}, true)
	if _, out := admin.do("GET", "/api/activity/status", nil, false); out["on"] != false {
		t.Fatalf("stopped: %v", out)
	}
	before := len(st.ActivityLog(100))
	admin.do("GET", "/api/books?q=after", nil, false)
	if len(st.ActivityLog(100)) != before {
		t.Fatal("nothing is kept after it stops")
	}
	admin.do("DELETE", "/api/admin/activity", nil, true)
	if n := len(st.ActivityLog(10)); n != 0 {
		t.Fatalf("deleted: %d", n)
	}
}
