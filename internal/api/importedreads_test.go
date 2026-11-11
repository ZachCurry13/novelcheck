package api_test

import (
	"testing"
)

// A list's "read" shelf (Goodreads) brings each book's finish day, pages and stars; Reading stats gets them as
// imported_read, and the stars become the importer's rating. A bad day is dropped, not the book.
func TestImportedReadsReachReadingStats(t *testing.T) {
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	res, _ := admin.do("POST", "/api/import/drive", map[string]any{
		"catalog_name": "From Goodreads",
		"books": []map[string]any{
			{"title": "Cinder", "author": "Marissa Meyer", "format": "list", "path": "list:Cinder | Marissa Meyer", "read": true, "finished": "2026-01-02", "pages": 390, "rating": 4, "binding": "audio", "reads": 2},
			{"title": "No Day", "author": "A. Writer", "format": "list", "path": "list:No Day | A. Writer", "read": true, "finished": "sometime", "pages": 120},
			{"title": "Unread", "author": "B. Writer", "format": "list", "path": "list:Unread | B. Writer", "finished": "2026-02-02"},
		},
	}, true)
	if res.StatusCode != 200 {
		t.Fatalf("import: %d", res.StatusCode)
	}
	_, stats := admin.do("GET", "/api/me/reading-stats", nil, false)
	list, _ := stats["imported_read"].([]any)
	if len(list) != 1 {
		t.Fatalf("only the finished book with a real day is listed: %v", stats["imported_read"])
	}
	one := list[0].(map[string]any)
	if one["title"] != "Cinder" || one["finished"] != "2026-01-02" || one["pages"] != float64(390) || one["rating"] != float64(4) || one["format"] != "audio" || one["reads"] != float64(2) {
		t.Fatalf("the finish day, pages and stars come through: %v", one)
	}
}
