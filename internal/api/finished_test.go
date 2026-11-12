package api_test

import (
	"testing"
)

// "I read this" puts a book among your finished books on a day (it counts in Reading stats), "Not read" takes it
// out, and a list's "read" book with no day is listed so a day can be given.
func TestMarkBookFinished(t *testing.T) {
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	res, _ := admin.do("POST", "/api/import/drive", map[string]any{
		"catalog_name": "From Goodreads",
		"books": []map[string]any{
			{"title": "My Phony Valentine", "author": "Courtney Walsh", "format": "list", "path": "list:My Phony Valentine | Courtney Walsh", "read": true},
		},
	}, true)
	if res.StatusCode != 200 {
		t.Fatalf("import: %d", res.StatusCode)
	}
	_, stats := admin.do("GET", "/api/me/reading-stats", nil, false)
	undated, _ := stats["imported_undated"].([]any)
	if len(undated) != 1 || stats["imported_read"] != nil {
		t.Fatalf("the book with no day is listed as undated: %v", stats)
	}
	id := int64(undated[0].(map[string]any)["book_id"].(float64))
	path := "/api/books/" + itoa(id) + "/finished"
	if res, _ := admin.do("PUT", path, map[string]string{"day": "2999-01-01"}, true); res.StatusCode != 400 {
		t.Fatalf("a day in the future is refused: %d", res.StatusCode)
	}
	if res, out := admin.do("PUT", path, map[string]string{"day": "2026-02-14"}, true); res.StatusCode != 200 || out["day"] != "2026-02-14" {
		t.Fatalf("mark read: %d %v", res.StatusCode, out)
	}
	_, stats = admin.do("GET", "/api/me/reading-stats", nil, false)
	if read, _ := stats["imported_read"].([]any); len(read) != 1 || stats["imported_undated"] != nil {
		t.Fatalf("now it counts on its day: %v", stats)
	}
	if res, out := admin.do("PUT", path, map[string]string{"day": ""}, true); res.StatusCode != 200 || out["day"] != "" {
		t.Fatalf("not read: %d %v", res.StatusCode, out)
	}
	_, stats = admin.do("GET", "/api/me/reading-stats", nil, false)
	if stats["imported_read"] != nil || stats["imported_undated"] != nil {
		t.Fatalf("taken out again: %v", stats)
	}
}
