package api_test

import (
	"net/http"
	"testing"
)

// An admin sees the app as another person, read-only, and goes back to their own account.
func TestViewAs(t *testing.T) {
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	if res, _ := admin.do("POST", "/api/admin/users", map[string]string{"username": "kiddo", "password": "kiddopass1", "role": "restricted"}, true); res.StatusCode != 201 {
		t.Fatalf("kid: %d", res.StatusCode)
	}
	_, me := admin.do("GET", "/api/me", nil, false)
	adminID := me["id"].(float64)
	_, users := admin.doList("GET", "/api/admin/users")
	var kid float64
	for _, u := range users {
		if u["username"] == "kiddo" {
			kid = u["id"].(float64)
		}
	}
	if res, _ := admin.do("POST", sprintf("/api/admin/users/%d/view-as", int(adminID)), nil, true); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("viewing as an admin: %d", res.StatusCode)
	}
	if res, out := admin.do("POST", sprintf("/api/admin/users/%d/view-as", int(kid)), nil, true); res.StatusCode != 200 {
		t.Fatalf("view as: %d %v", res.StatusCode, out)
	}
	_, me = admin.do("GET", "/api/me", nil, false)
	if me["username"] != "kiddo" || me["viewed_by"] != "admin" {
		t.Fatalf("seeing the app as the kid: %v", me)
	}
	if res, _ := admin.do("GET", "/api/books", nil, false); res.StatusCode != 200 {
		t.Fatalf("looking around: %d", res.StatusCode)
	}
	if res, _ := admin.do("PUT", "/api/me/password", map[string]string{"current": "x", "password": "newpass123"}, true); res.StatusCode != http.StatusForbidden {
		t.Fatalf("a change while viewing: %d", res.StatusCode)
	}
	if res, _ := admin.do("GET", "/api/admin/users", nil, false); res.StatusCode == 200 {
		t.Fatal("the kid's view reached the admin pages")
	}
	if res, _ := admin.do("DELETE", "/api/me/view-as", nil, true); res.StatusCode != 200 {
		t.Fatalf("back: %d", res.StatusCode)
	}
	_, me = admin.do("GET", "/api/me", nil, false)
	if me["username"] != "admin" || me["viewed_by"] != nil {
		t.Fatalf("back to the admin: %v", me)
	}
}

// Viewing as a friend (whose account refuses most of the app) still lets the admin go back.
func TestViewAsAFriendAndBack(t *testing.T) {
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	_, made := admin.do("POST", "/api/admin/users", map[string]string{"username": "pal", "password": "palpass123", "role": "friend"}, true)
	if res, out := admin.do("POST", sprintf("/api/admin/users/%d/view-as", int(made["id"].(float64))), nil, true); res.StatusCode != 200 {
		t.Fatalf("view as: %d %v", res.StatusCode, out)
	}
	if res, out := admin.do("DELETE", "/api/me/view-as", nil, true); res.StatusCode != 200 {
		t.Fatalf("back from a friend's view: %d %v", res.StatusCode, out)
	}
	if _, me := admin.do("GET", "/api/me", nil, false); me["username"] != "admin" {
		t.Fatalf("still viewing: %v", me)
	}
}
