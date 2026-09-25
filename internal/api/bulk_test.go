package api_test

import (
	"fmt"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestBulkBookActions(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]string{"username": "reader", "password": "editorpass1", "role": "editor"}, true)
	reader := login(t, srv, "reader", "editorpass1")
	_, out := reader.do("POST", "/api/import/drive", map[string]any{"catalog_name": "Reader's Kindle", "books": []map[string]string{
		{"title": "Mine One", "format": "list", "path": "list:1"}, {"title": "Mine Two", "format": "list", "path": "list:2"}}}, true)
	if out["imported"].(float64) != 2 {
		t.Fatalf("import: %v", out)
	}
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	family, _ := st.UpsertBook("Family Book", "Someone", "", "")
	_ = st.AddCopy(cal, family, "/calibre/family.epub", "epub", "9")
	looked, _ := st.EnsureCatalog(store.LookedUpCatalog, "custom")
	lookup, _ := st.UpsertBook("Only Looked Up", "Nobody", "", "")
	_ = st.AddCopy(looked, lookup, "lookup:x", "list", "")
	ids := []int64{}
	_ = st.DB.Select(&ids, `SELECT id FROM books ORDER BY id`)

	res, q := reader.do("POST", "/api/books/bulk", map[string]any{"ids": ids, "action": "queue"}, true)
	if res.StatusCode != 200 || q["queued"].(float64) != 4 {
		t.Fatalf("queue: %d %v", res.StatusCode, q)
	}
	if _, d := reader.do("POST", "/api/books/bulk", map[string]any{"ids": ids, "action": "deep"}, true); d["no_epub"].(float64) != 4 {
		t.Fatalf("deep scan needs an EPUB file: %v", d)
	}
	_, del := reader.do("POST", "/api/books/bulk", map[string]any{"ids": ids, "action": "delete", "reason": "done with these"}, true)
	if del["removed"].(float64) != 2 || del["requested"].(float64) != 1 || del["skipped"].(float64) != 1 {
		t.Fatalf("own books go at once, the family's are requested, looked-up ones skipped: %v", del)
	}
	if _, b := reader.do("GET", fmt.Sprintf("/api/books/%d", family), nil, false); b["my_delete_request"] == nil {
		t.Fatal("a delete request for the family's book")
	}
	if res, _ := reader.do("POST", "/api/books/bulk", map[string]any{"ids": ids, "action": "explode"}, true); res.StatusCode != 400 {
		t.Fatal("unknown action")
	}
}
