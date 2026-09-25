package api_test

import (
	"fmt"
	"testing"
)

func TestPrivateLibraries(t *testing.T) {
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	for _, u := range []string{"owner", "other"} {
		admin.do("POST", "/api/admin/users", map[string]string{"username": u, "password": "editorpass1", "role": "editor"}, true)
	}
	admin.do("POST", "/api/admin/users", map[string]string{"username": "kid", "password": "kidpass12"}, true)
	owner, other, kid := login(t, srv, "owner", "editorpass1"), login(t, srv, "other", "editorpass1"), login(t, srv, "kid", "kidpass12")

	res, out := owner.do("POST", "/api/import/drive", map[string]any{"catalog_name": "My Kindle", "private": true,
		"books": []map[string]string{{"title": "Secret Book", "author": "A. Writer", "format": "list", "path": "list:Secret Book"}}}, true)
	if res.StatusCode != 200 {
		t.Fatalf("import: %d %v", res.StatusCode, out)
	}
	cat := int64(out["catalog_id"].(float64))
	count := func(c *client) float64 {
		_, o := c.do("GET", "/api/books?q=Secret", nil, false)
		return o["total"].(float64)
	}
	if count(owner) != 1 || count(admin) != 1 || count(other) != 0 || count(kid) != 0 {
		t.Fatalf("private: owner %v admin %v other %v kid %v", count(owner), count(admin), count(other), count(kid))
	}
	if _, list := other.doList("GET", "/api/catalogs"); len(list) != 0 {
		t.Fatalf("others don't see a private library: %v", list)
	}
	if res, _ := other.do("POST", "/api/import/drive", map[string]any{"catalog_name": "my kindle", "books": []map[string]string{}}, true); res.StatusCode != 403 {
		t.Fatalf("importing into someone else's library: %d", res.StatusCode)
	}
	if res, _ := other.do("PATCH", fmt.Sprintf("/api/catalogs/%d", cat), map[string]bool{"private": false}, true); res.StatusCode != 403 {
		t.Fatal("only the owner shares it")
	}
	owner.do("PATCH", fmt.Sprintf("/api/catalogs/%d", cat), map[string]bool{"private": false}, true)
	if count(other) != 1 {
		t.Fatal("shared with everyone")
	}
	if res, _ := admin.do("PATCH", fmt.Sprintf("/api/catalogs/%d", cat), map[string]int64{"owner_id": 4}, true); res.StatusCode != 400 {
		t.Fatalf("kids can't own a library: %d", res.StatusCode)
	}

	// The owner removes a book from their own library straight away.
	_, o := owner.do("GET", "/api/books?q=Secret", nil, false)
	book := int64(o["books"].([]any)[0].(map[string]any)["id"].(float64))
	if res, _ := other.do("DELETE", fmt.Sprintf("/api/catalogs/%d/books/%d", cat, book), nil, true); res.StatusCode != 403 {
		t.Fatal("not someone else's")
	}
	if res, _ := owner.do("DELETE", fmt.Sprintf("/api/catalogs/%d/books/%d", cat, book), nil, true); res.StatusCode != 200 || count(owner) != 0 {
		t.Fatalf("removed: %d", res.StatusCode)
	}
	if res, _ := other.do("DELETE", fmt.Sprintf("/api/catalogs/%d", cat), nil, true); res.StatusCode != 403 {
		t.Fatal("only the owner or an admin deletes a library")
	}
	if res, _ := owner.do("DELETE", fmt.Sprintf("/api/catalogs/%d", cat), nil, true); res.StatusCode != 200 {
		t.Fatalf("owner deletes it: %d", res.StatusCode)
	}
}
