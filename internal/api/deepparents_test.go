package api_test

import "testing"

// Parents (admins and editors) can read a scanned part in the book window;
// kids can't.
func TestDeepPassageIsForParents(t *testing.T) {
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]string{"username": "parent", "password": "editorpass1", "role": "editor"}, true)
	admin.do("POST", "/api/admin/users", map[string]any{"username": "tween", "password": "kidpass12", "age_level": 2}, true)
	editor := login(t, srv, "parent", "editorpass1")
	kid := login(t, srv, "tween", "kidpass12")
	if res, _ := kid.do("GET", "/api/admin/deep-scans/999/passage?from=0&to=10", nil, false); res.StatusCode != 403 {
		t.Fatalf("kid: %d", res.StatusCode)
	}
	if res, _ := editor.do("GET", "/api/admin/deep-scans/999/passage?from=0&to=10", nil, false); res.StatusCode != 404 {
		t.Fatalf("editor should get past the role check (no such scan): %d", res.StatusCode)
	}
}
