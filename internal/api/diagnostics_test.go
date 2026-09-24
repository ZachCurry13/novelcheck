package api_test

import (
	"io"
	"strings"
	"testing"
)

// The diagnostics report must never contain secrets, and hides most of any
// email address.
func TestDiagnosticsHasNoSecrets(t *testing.T) {
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("PUT", "/api/admin/settings", map[string]string{
		"llm_api_key": "sk-supersecret123", "smtp_password": "app-pass-abcd", "smtp_username": "reader@gmail.com",
		"google_books_api_key": "AIzaSecretKey"}, true)

	res, err := admin.http.Get(admin.base + "/api/admin/diagnostics")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	text := string(raw)
	if res.StatusCode != 200 || !strings.Contains(text, "NovelCheck diagnostics") || !strings.Contains(text, "llm_api_key = (set)") {
		t.Fatalf("report: %d\n%s", res.StatusCode, text)
	}
	for _, secret := range []string{"sk-supersecret123", "app-pass-abcd", "AIzaSecretKey", "reader@gmail.com"} {
		if strings.Contains(text, secret) {
			t.Fatalf("diagnostics leaked %q", secret)
		}
	}
	if !strings.Contains(text, "r***@gmail.com") {
		t.Fatal("email should be partly shown")
	}

	admin.do("POST", "/api/admin/users", map[string]string{"username": "mom", "password": "mompass123", "role": "editor"}, true)
	mom := login(t, srv, "mom", "mompass123")
	if res, _ := mom.do("GET", "/api/admin/diagnostics", nil, false); res.StatusCode != 403 {
		t.Fatalf("diagnostics are admin-only: %d", res.StatusCode)
	}
}
