package api_test

import (
	"fmt"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestTasteProfile(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	_ = st.SetSetting(store.KeySuggestMode, store.SuggestFree)
	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	ids := map[string]int64{}
	for i, title := range []string{"Guards! Guards! (Discworld, #8)", "Men at Arms (Discworld, #9)", "Emma"} {
		id, _ := st.UpsertBook(title, map[bool]string{true: "Jane Austen", false: "Terry Pratchett"}[title == "Emma"], "", "")
		_ = st.AddCopy(cat, id, fmt.Sprintf("/calibre/%d.epub", i), "epub", fmt.Sprint(i+1))
		ids[title] = id
	}
	guards := ids["Guards! Guards! (Discworld, #8)"]

	_, out := admin.do("GET", "/api/taste", nil, false)
	if len(out["books"].([]any)) != 3 || len(out["marks"].([]any)) != 0 { // a small library: every book
		t.Fatalf("books to mark: %v", out)
	}
	if res, _ := admin.do("POST", "/api/taste", map[string]any{"book_id": guards, "mark": "bogus"}, true); res.StatusCode != 400 {
		t.Fatal("unknown answer")
	}
	// "Read & liked" is reading history: the series goes on from there.
	admin.do("POST", "/api/taste", map[string]any{"book_id": guards, "mark": "liked"}, true)
	_, sg := admin.do("GET", "/api/suggestions", nil, false)
	items := sg["items"].([]any)
	if len(items) == 0 || items[0].(map[string]any)["reason"] != "Next in Discworld after Guards! Guards!" || sg["up"].(float64) != 1 {
		t.Fatalf("suggestions from the taste profile: %v", sg)
	}
	_, out = admin.do("GET", "/api/taste", nil, false)
	marks := out["marks"].([]any)
	if len(marks) != 1 || marks[0].(map[string]any)["mark"] != "liked" {
		t.Fatalf("answers so far: %v", out)
	}
	for _, b := range out["books"].([]any) {
		if b.(map[string]any)["id"].(float64) == float64(guards) {
			t.Fatal("a marked book is offered again")
		}
	}
	// Clearing an answer; and the admin's switch.
	admin.do("POST", "/api/taste", map[string]any{"book_id": guards, "mark": ""}, true)
	if _, out := admin.do("GET", "/api/taste", nil, false); len(out["marks"].([]any)) != 0 {
		t.Fatal("cleared")
	}
	admin.do("PUT", "/api/admin/settings", map[string]string{"module_taste": "false"}, true)
	if res, _ := admin.do("GET", "/api/taste", nil, false); res.StatusCode != 403 {
		t.Fatalf("switched off: %d", res.StatusCode)
	}
}
