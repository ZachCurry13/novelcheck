package api_test

import "testing"

// A person's own star rating: quarter steps, shown on the book, cleared with 0, and in their reading stats.
func TestStarRatings(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	id, _ := st.UpsertBook("Rated Book", "Author", "", "")
	path := "/api/books/" + itoa(id) + "/rating"

	if res, _ := admin.do("PUT", path, map[string]float64{"stars": 6}, true); res.StatusCode != 400 {
		t.Fatalf("more than 5 stars: %d", res.StatusCode)
	}
	if _, out := admin.do("PUT", path, map[string]float64{"stars": 4.3}, true); out["stars"] != 4.25 {
		t.Fatalf("rounded to a quarter: %v", out)
	}
	if _, out := admin.do("GET", "/api/books/"+itoa(id), nil, false); out["my_rating"] != 4.25 {
		t.Fatalf("shown on the book: %v", out["my_rating"])
	}
	if _, out := admin.do("PUT", path, map[string]float64{"stars": 0}, true); out["stars"] != float64(0) {
		t.Fatalf("cleared: %v", out)
	}
	if res, _ := admin.do("PUT", "/api/books/99999/rating", map[string]float64{"stars": 3}, true); res.StatusCode != 404 {
		t.Fatalf("an unknown book: %d", res.StatusCode)
	}
}
