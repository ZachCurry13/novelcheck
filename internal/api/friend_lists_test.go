package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// A New York Times book the family owns is still on a friend's Discover, and
// its page opens, but nothing of the family's copy shows and no file door opens.
func TestAFriendSeesTheWholeBestSellerList(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	if res, _ := admin.do("POST", "/api/admin/users", map[string]string{"username": "pal", "password": "palpass123", "role": "friend"}, true); res.StatusCode != 201 {
		t.Fatalf("friend: %d", res.StatusCode)
	}
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	ours, _ := st.UpsertBook("Best Seller We Own", "Cy Penn", "", "")
	_ = st.AddCopy(cal, ours, "/c/best.epub", "epub", "2")
	if _, err := st.SaveDiscoverList("nyt:combined-print-and-e-book-fiction", []store.DiscoverEntry{{Rank: 1, Title: "Best Seller We Own", Author: "Cy Penn"}}); err != nil {
		t.Fatal(err)
	}
	pal := login(t, srv, "pal", "palpass123")

	res, body := pal.do("GET", "/api/discover", nil, false)
	if res.StatusCode != 200 {
		t.Fatalf("discover: %d", res.StatusCode)
	}
	var card map[string]any
	for _, r := range body["rows"].([]any) {
		for _, b := range r.(map[string]any)["books"].([]any) {
			if b.(map[string]any)["id"].(float64) == float64(ours) {
				card = b.(map[string]any)
			}
		}
	}
	if card == nil || card["owned"] != false || card["formats"] != "" || body["owned"].(float64) != 0 {
		t.Fatalf("the family's best seller on her Discover: %v (owned %v)", card, body["owned"])
	}

	res, page := pal.do("GET", sprintf("/api/books/%d", ours), nil, false)
	if res.StatusCode != 200 || len(page["copies"].([]any)) != 0 || page["downloadable"] != false || page["owned"] != false ||
		page["book"].(map[string]any)["formats"] != "" {
		t.Fatalf("its page: %d %v", res.StatusCode, page)
	}
	for _, p := range []string{"/api/books/%d/download", "/api/books/%d/listen", "/api/books/%d/send"} {
		if res, _ := pal.do("GET", sprintf(p, ours), nil, false); res.StatusCode == 200 {
			t.Fatalf("%s opened", p)
		}
	}
	if res, body := pal.do("GET", "/api/books?q=Best", nil, false); res.StatusCode != 200 || len(body["books"].([]any)) != 0 {
		t.Fatalf("the family's copy in her Library search: %v", body)
	}
}
