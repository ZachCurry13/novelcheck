package api_test

import (
	"strings"
	"testing"
)

// The main admin decides what another admin may reach; the server refuses
// the rest, settings included.
func TestAdminAreas(t *testing.T) {
	srv, _ := setup(t)
	owner := login(t, srv, "admin", "adminpass1")
	if _, me := owner.do("GET", "/api/me", nil, false); me["owner"] != true || len(me["areas"].([]any)) != 6 {
		t.Fatalf("the first admin is the main admin with every area: %v %v", me["owner"], me["areas"])
	}
	_, partner := owner.do("POST", "/api/admin/users", map[string]string{"username": "partner", "password": "partnerpass1", "role": "admin"}, true)
	pid := itoa(int64(partner["id"].(float64)))
	owner.do("POST", "/api/admin/users", map[string]string{"username": "editor1", "password": "editorpass1", "role": "editor"}, true)
	owner.do("POST", "/api/admin/users", map[string]any{"username": "kid1", "password": "kidpass123", "age_level": 2}, true)

	p := login(t, srv, "partner", "partnerpass1")
	_, me := p.do("GET", "/api/me", nil, false)
	if me["owner"] != false || len(me["areas"].([]any)) != 1 || me["areas"].([]any)[0] != "deep" {
		t.Fatalf("a new admin gets Deep Scans only: %v", me["areas"])
	}
	status := func(c *client, method, path string, body any) int {
		res, _ := c.do(method, path, body, method != "GET")
		return res.StatusCode
	}
	for path, want := range map[string]int{"/api/admin/deep-scans": 200, "/api/admin/aitools": 403, "/api/admin/system": 403,
		"/api/admin/formats": 403, "/api/admin/discover": 403, "/api/admin/ollama/models": 403} {
		if got := status(p, "GET", path, nil); got != want {
			t.Errorf("%s: %d, want %d", path, got, want)
		}
	}

	// Settings: only Deep Scan ones are shown, and AI ones can't be saved.
	_, set := p.do("GET", "/api/admin/settings", nil, false)
	if _, ok := set["llm_model"]; ok {
		t.Error("AI settings shown to an admin without AI settings")
	}
	if _, ok := set["deep_scan_top_n"]; !ok {
		t.Error("Deep Scan settings missing")
	}
	if got := status(p, "PUT", "/api/admin/settings", map[string]string{"llm_model": "x"}); got != 403 {
		t.Errorf("saving an AI setting: %d", got)
	}
	if got := status(p, "PUT", "/api/admin/settings", map[string]string{"deep_scan_top_n": "20"}); got != 200 {
		t.Errorf("saving a Deep Scan setting: %d", got)
	}

	// Users: kids only; no parents or admins; no changing access.
	_, list := p.doList("GET", "/api/admin/users")
	for _, u := range list {
		if u["role"] != "restricted" {
			t.Errorf("an admin without Users sees %v", u["username"])
		}
	}
	if got := status(p, "POST", "/api/admin/users", map[string]string{"username": "e2", "password": "editorpass1", "role": "editor"}); got != 403 {
		t.Errorf("creating a parent: %d", got)
	}
	if got := status(p, "POST", "/api/admin/users", map[string]any{"username": "kid2", "password": "kidpass123", "age_level": 1}); got != 201 {
		t.Errorf("creating a kid: %d", got)
	}
	if got := status(p, "PUT", "/api/admin/users/"+pid+"/access", map[string]any{"areas": []string{"ai"}}); got != 403 {
		t.Errorf("giving yourself areas: %d", got)
	}

	// The main admin gives AI settings: now they work.
	if res, out := owner.do("PUT", "/api/admin/users/"+pid+"/access", map[string]any{"areas": []string{"deep", "ai", "bogus"}}, true); res.StatusCode != 200 ||
		strings.Join(toStrings(out["areas"]), ",") != "ai,deep" {
		t.Fatalf("access: %d %v", res.StatusCode, out)
	}
	if got := status(p, "GET", "/api/admin/aitools", nil); got != 200 {
		t.Errorf("AI settings after the main admin allowed it: %d", got)
	}

	// The main admin can't be demoted or deleted; it's handed over instead.
	_, ownerMe := owner.do("GET", "/api/me", nil, false)
	oid := itoa(int64(ownerMe["id"].(float64)))
	if got := status(p, "DELETE", "/api/admin/users/"+oid, nil); got != 403 {
		t.Errorf("an admin deleting the main admin: %d", got)
	}
	if res, _ := owner.do("POST", "/api/admin/users/"+pid+"/owner", nil, true); res.StatusCode != 200 {
		t.Fatalf("hand over: %d", res.StatusCode)
	}
	if _, me := p.do("GET", "/api/me", nil, false); me["owner"] != true {
		t.Fatalf("new main admin: %v", me)
	}
	if _, me := owner.do("GET", "/api/me", nil, false); me["owner"] != false || len(me["areas"].([]any)) != 6 {
		t.Fatalf("the old main admin keeps every area: %v", me)
	}
}

func toStrings(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}
