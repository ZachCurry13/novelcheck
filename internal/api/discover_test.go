package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestDiscoverAPI(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	_, _ = st.SaveDiscoverList("ol:classics", []store.DiscoverEntry{{Rank: 1, Title: "Emma", Author: "Jane Austen", Link: "https://openlibrary.org/works/OL1W"}})

	res, out := admin.do("GET", "/api/discover", nil, false)
	rows, _ := out["rows"].([]any)
	if res.StatusCode != 200 || len(rows) != 1 || out["nyt"] != false || out["has_key"] != false {
		t.Fatalf("discover: %d %v", res.StatusCode, out)
	}
	book := rows[0].(map[string]any)["books"].([]any)[0].(map[string]any)
	if book["title"] != "Emma" || book["link"] != "https://openlibrary.org/works/OL1W" || book["owned"] != false {
		t.Fatalf("book: %v", book)
	}
	if res, s := admin.do("GET", "/api/admin/discover", nil, false); res.StatusCode != 200 || s["counts"].(map[string]any)["listed"].(float64) != 1 {
		t.Fatalf("status: %d %v", res.StatusCode, s)
	}
	if res, _ := admin.do("POST", "/api/admin/discover/test", map[string]string{"key": ""}, true); res.StatusCode != 400 {
		t.Fatalf("a test without a key: %d", res.StatusCode)
	}
	// Wishing for a Discover book works (it isn't owned).
	id := int64(book["id"].(float64))
	if res, _ := admin.do("POST", "/api/books/"+itoa(id)+"/wish", map[string]string{}, true); res.StatusCode >= 300 {
		t.Fatalf("wish: %d", res.StatusCode)
	}
	_ = st.SetSetting(store.KeyModuleDiscover, "false")
	if res, _ := admin.do("GET", "/api/discover", nil, false); res.StatusCode != 403 {
		t.Fatalf("switched off: %d", res.StatusCode)
	}
}
