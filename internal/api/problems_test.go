package api_test

import "testing"

func TestReportProblemToAdmin(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]string{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	if res, _ := kid.do("POST", "/api/problems", map[string]string{"text": "  "}, true); res.StatusCode != 400 {
		t.Fatal("empty report")
	}
	if res, _ := kid.do("POST", "/api/problems", map[string]string{"text": "The Up Next page is blank", "page": "#/queue"}, true); res.StatusCode != 200 {
		t.Fatalf("report: %d", res.StatusCode)
	}
	if res, _ := kid.do("GET", "/api/admin/problems", nil, false); res.StatusCode != 403 {
		t.Fatal("only admins read reports")
	}
	_, s := admin.do("GET", "/api/admin/status", nil, false)
	_, out := admin.do("GET", "/api/admin/problems", nil, false)
	r := out["reports"].([]any)[0].(map[string]any)
	if s["problem_reports"].(float64) != 1 || r["username"] != "kid" || r["page"] != "#/queue" || r["device"] == "" {
		t.Fatalf("admin sees it: %v %v", s["problem_reports"], out)
	}
	if res, _ := admin.do("POST", "/api/admin/problems/"+itoa(int64(r["id"].(float64)))+"/done", nil, true); res.StatusCode != 200 || st.CountProblemReports() != 0 {
		t.Fatalf("done: %d", res.StatusCode)
	}
}
