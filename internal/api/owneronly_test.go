package api_test

import (
	"fmt"
	"strings"
	"testing"
)

// Only the main admin deletes a book's real files; another admin gets a clear refusal (and can still dismiss requests).
func TestOnlyTheMainAdminDeletesFiles(t *testing.T) {
	srv, _ := setup(t)
	owner := login(t, srv, "admin", "adminpass1")
	_, p := owner.do("POST", "/api/admin/users", map[string]string{"username": "partner", "password": "partnerpass1", "role": "admin"}, true)
	owner.do("PUT", "/api/admin/users/"+fmt.Sprint(p["id"])+"/access", map[string]any{"areas": []string{"ai", "calibre"}}, true) // every area the library tools sit in
	other := login(t, srv, "partner", "partnerpass1")
	for path, body := range map[string]any{
		"/api/admin/calibre/remove":            map[string]any{"book_ids": []int64{1}},
		"/api/admin/calibre/duplicates/remove": map[string]any{"ids": []int{1}},
		"/api/admin/formats/remove":            map[string]any{"files": []any{}},
		"/api/admin/delete-requests/decide":    map[string]any{"book_ids": []int64{1}, "action": "delete"},
	} {
		if res, out := other.do("POST", path, body, true); res.StatusCode != 403 || !strings.Contains(fmt.Sprint(out["error"]), "only the main admin") {
			t.Fatalf("%s: another admin is refused: %d %v", path, res.StatusCode, out)
		}
	}
	if res, _ := other.do("POST", "/api/admin/delete-requests/decide", map[string]any{"book_ids": []int64{1}, "action": "dismiss"}, true); res.StatusCode == 403 {
		t.Fatal("another admin can still dismiss a request")
	}
	if res, _ := owner.do("POST", "/api/admin/calibre/remove", map[string]any{"book_ids": []int64{}}, true); res.StatusCode == 403 {
		t.Fatal("the main admin is let through")
	}
}
