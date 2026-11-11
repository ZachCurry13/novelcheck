package api_test

import (
	"testing"
)

// The Integrations page lists the parts and services in groups, and the database check answers.
func TestIntegrationsPage(t *testing.T) {
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	res, out := admin.do("GET", "/api/admin/integrations", nil, false)
	if res.StatusCode != 200 {
		t.Fatalf("integrations: %d", res.StatusCode)
	}
	groups, _ := out["groups"].([]any)
	if len(groups) != 4 {
		t.Fatalf("four groups: %v", out)
	}
	seen := map[string]bool{}
	for _, g := range groups {
		for _, r := range g.(map[string]any)["rows"].([]any) {
			row := r.(map[string]any)
			seen[row["id"].(string)] = true
			if row["name"] == "" || row["label"] == "" {
				t.Fatalf("a row has a name and a status: %v", row)
			}
		}
	}
	for _, id := range []string{"updates", "database", "tunnel", "plugin", "calibre", "calibre-server", "audiobookshelf", "hardcover", "smtp", "openlibrary", "googlebooks", "discover"} {
		if !seen[id] {
			t.Errorf("missing row %q", id)
		}
	}
	if res, out := admin.do("POST", "/api/admin/integrations/db-check", map[string]string{}, true); res.StatusCode != 200 || out["ok"] != true {
		t.Fatalf("database check: %d %v", res.StatusCode, out)
	}
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	if res, _ := kid.do("GET", "/api/admin/integrations", nil, false); res.StatusCode == 200 {
		t.Fatal("not for kids")
	}
}
