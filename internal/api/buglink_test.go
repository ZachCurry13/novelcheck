package api_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// The public bug-report link: off by default, an admin turns it on, anyone
// holding the secret reads the reports as text, a new secret replaces the old
// one, and turning it off closes it. Browsers can send their own report.
func TestBugLink(t *testing.T) {
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	read := func(path string) (int, string) {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	if code, _ := read("/bugs/anything"); code != 404 {
		t.Fatalf("off by default: %d", code)
	}
	if res, _ := admin.do("POST", "/api/me/bug-report", map[string]string{"text": "tapped Save, got a 500"}, true); res.StatusCode != 200 {
		t.Fatalf("browser report: %d", res.StatusCode)
	}
	_, out := admin.do("POST", "/api/admin/bug-link", map[string]string{}, true)
	url, _ := out["url"].(string)
	if out["enabled"] != true || !strings.Contains(url, "/bugs/") {
		t.Fatalf("turned on: %v", out)
	}
	path := url[strings.Index(url, "/bugs/"):]
	code, text := read(path)
	if code != 200 || !strings.Contains(text, "NovelCheck diagnostics") || !strings.Contains(text, "tapped Save, got a 500") {
		t.Fatalf("the link shows the server report and the browser report: %d %q", code, text)
	}
	if strings.Contains(text, strings.TrimPrefix(path, "/bugs/")) {
		t.Fatal("the secret must not appear in its own report")
	}
	_, out = admin.do("POST", "/api/admin/bug-link", map[string]string{}, true)
	if code, _ := read(path); code != 404 || out["url"] == url {
		t.Fatalf("a new link replaces the old: %d", code)
	}
	admin.do("DELETE", "/api/admin/bug-link", nil, true)
	if code, _ := read(strings.Replace(path, path[6:], "x", 1)); code != 404 {
		t.Fatal("off again")
	}
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	if res, _ := kid.do("POST", "/api/admin/bug-link", map[string]string{}, true); res.StatusCode != 403 {
		t.Fatalf("only admins make the link: %d", res.StatusCode)
	}
}
