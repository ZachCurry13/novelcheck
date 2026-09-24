package api_test

import (
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestEditorPermissions(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	if res, _ := admin.do("POST", "/api/admin/users", map[string]string{"username": "wife", "password": "editorpass1", "role": "editor"}, true); res.StatusCode != 201 {
		t.Fatalf("admin create editor: %d", res.StatusCode)
	}
	ed := login(t, srv, "wife", "editorpass1")

	// Allowed: dashboard, kid accounts, scans, verdict edits.
	if res, _ := ed.do("GET", "/api/admin/status", nil, false); res.StatusCode != 200 {
		t.Fatalf("editor status: %d", res.StatusCode)
	}
	res, kid := ed.do("POST", "/api/admin/users", map[string]string{"username": "kid", "password": "kidpass12"}, true)
	if res.StatusCode != 201 || kid["role"] != "restricted" {
		t.Fatalf("editor create kid: %d %v", res.StatusCode, kid)
	}
	kidID := itoa(int64(kid["id"].(float64)))
	if res, _ := ed.do("PUT", "/api/admin/users/"+kidID, map[string]any{"role": "restricted", "delivery_method": "none", "hide_lgbtq": true}, true); res.StatusCode != 200 {
		t.Fatalf("editor update kid: %d", res.StatusCode)
	}
	id, _ := st.UpsertBook("Some Book", "An Author", "", "")
	res, b := ed.do("PUT", "/api/books/"+itoa(id)+"/verdict", map[string]any{"classification": "Closed Door", "heavy_innuendo": true, "summary_verdict": "Fine for teens."}, true)
	if res.StatusCode != 200 || b["classification"] != "Closed Door" || b["analysis_model"] != "manual: wife" {
		t.Fatalf("editor verdict: %d %v", res.StatusCode, b)
	}

	// Forbidden: technical settings, secrets, backups, admins, promotions.
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/admin/settings"},
		{"PUT", "/api/admin/settings"},
		{"GET", "/api/admin/backup"},
		{"GET", "/api/admin/tunnel"},
		{"GET", "/api/admin/calibre/server"},
		{"PUT", "/api/admin/calibre/server"},
		{"POST", "/api/admin/calibre/remove"},
		{"PUT", "/api/admin/tunnel"},
		{"POST", "/api/admin/wipe-queue"},
		{"GET", "/api/admin/calibre/browse"},
		{"PUT", "/api/admin/calibre/library"},
		{"PUT", "/api/admin/users/1"},          // the admin
		{"PUT", "/api/admin/users/1/password"}, // the admin
		{"DELETE", "/api/admin/users/1"},
	} {
		if res, _ := ed.do(c.method, c.path, map[string]string{}, true); res.StatusCode != 403 {
			t.Errorf("editor %s %s: got %d, want 403", c.method, c.path, res.StatusCode)
		}
	}
	if res, _ := ed.do("POST", "/api/admin/users", map[string]string{"username": "boss", "password": "bosspass12", "role": "admin"}, true); res.StatusCode != 403 {
		t.Fatalf("editor created an admin: %d", res.StatusCode)
	}
	if res, _ := ed.do("PUT", "/api/admin/users/"+kidID, map[string]any{"role": "admin", "delivery_method": "none"}, true); res.StatusCode != 403 {
		t.Fatalf("editor promoted a kid: %d", res.StatusCode)
	}
	res, list := ed.doList("GET", "/api/admin/users")
	if res.StatusCode != 200 || len(list) != 1 || list[0]["role"] != store.RoleRestricted {
		t.Fatalf("editor should only see kid accounts: %v", list)
	}

	// Kids still can't reach any management route.
	k := login(t, srv, "kid", "kidpass12")
	if res, _ := k.do("PUT", "/api/books/"+itoa(id)+"/verdict", map[string]any{"classification": "No Spice"}, true); res.StatusCode != 403 {
		t.Fatalf("kid edited a verdict: %d", res.StatusCode)
	}
}

func TestTunnelSettings(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cmd := "sudo cloudflared service install eyJhIjoiYWJjIn0="
	res, out := admin.do("PUT", "/api/admin/tunnel", map[string]any{"token": cmd, "hostname": "https://books.example.com/", "enabled": false}, true)
	if res.StatusCode != 200 {
		t.Fatalf("save: %d %v", res.StatusCode, out)
	}
	if got := st.Setting(store.KeyTunnelToken); got != "eyJhIjoiYWJjIn0=" {
		t.Fatalf("token not extracted from command: %q", got)
	}
	if got := st.Setting(store.KeyTunnelHostname); got != "books.example.com" {
		t.Fatalf("hostname not cleaned: %q", got)
	}
	// Enabling without the connector installed gives a clear error.
	if res, out := admin.do("PUT", "/api/admin/tunnel", map[string]any{"enabled": true}, true); res.StatusCode != 400 || !strings.Contains(out["error"].(string), "not installed") {
		t.Fatalf("expected not-installed error: %d %v", res.StatusCode, out)
	}
	// The token is a secret: never returned by the settings API.
	_, settings := admin.do("GET", "/api/admin/settings", nil, false)
	if _, ok := settings["tunnel_token"]; ok {
		t.Fatal("tunnel token exposed in settings")
	}
	_, status := admin.do("GET", "/api/admin/tunnel", nil, false)
	if status["has_token"] != true || strings.Contains(status["guide"].(string), "eyJhIjoiYWJjIn0") {
		t.Fatalf("unexpected tunnel status %v", status["has_token"])
	}
}

func TestApprovalAndRemovalAPI(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	id, _ := st.UpsertBook("Harry Potter", "J. K. Rowling", "", "")
	_ = st.AddCopy(cal, id, "/calibre/hp.epub", "epub", "42")
	_ = st.SaveAnalysis(id, store.Analysis{Classification: "No Spice", DarkOccult: true})

	res, out := admin.do("GET", "/api/admin/calibre/removal?hide=dark_occult", nil, false)
	if res.StatusCode != 200 || out["search"] != "id:=42" {
		t.Fatalf("removal: %d %v", res.StatusCode, out)
	}
	if res, _ := admin.do("GET", "/api/admin/calibre/removal", nil, false); res.StatusCode != 400 {
		t.Fatalf("removal without filters should be refused: %d", res.StatusCode)
	}
	res, b := admin.do("PUT", "/api/books/"+itoa(id)+"/approval", map[string]bool{"approved": true}, true)
	if res.StatusCode != 200 || b["approved"] != true || b["approved_by"] != "admin" {
		t.Fatalf("approve: %d %v", res.StatusCode, b)
	}
	if _, out := admin.do("GET", "/api/admin/calibre/removal?hide=dark_occult", nil, false); out["count"].(float64) != 0 {
		t.Fatalf("approved book offered for removal: %v", out)
	}

	admin.do("POST", "/api/admin/users", map[string]string{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	if res, _ := kid.do("PUT", "/api/books/"+itoa(id)+"/approval", map[string]bool{"approved": false}, true); res.StatusCode != 403 {
		t.Fatalf("kid changed an approval: %d", res.StatusCode)
	}
	if res, _ := kid.do("GET", "/api/books/"+itoa(id), nil, false); res.StatusCode != 200 {
		t.Fatalf("kid should see the approved book: %d", res.StatusCode)
	}
}
