package api_test

import "testing"

// A book being read can go back to the top of the Nightstand without being removed.
func TestPauseReading(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	a, _ := st.UpsertBook("First Book", "Author", "", "")
	b, _ := st.UpsertBook("Second Book", "Author", "", "")
	admin.do("POST", "/api/queue", map[string]int64{"book_id": a}, true)
	admin.do("POST", "/api/queue", map[string]int64{"book_id": b}, true)
	_, items := admin.doList("GET", "/api/queue")
	var firstItem float64
	for _, it := range items {
		if it["title"] == "First Book" {
			firstItem = it["id"].(float64)
		}
	}
	admin.do("POST", "/api/queue/"+itoa(int64(firstItem))+"/start", nil, true)
	if res, _ := admin.do("POST", "/api/queue/"+itoa(int64(firstItem))+"/pause", map[string]any{}, true); res.StatusCode != 200 {
		t.Fatalf("pause: %d", res.StatusCode)
	}
	_, items = admin.doList("GET", "/api/queue")
	if len(items) != 2 || items[0]["title"] != "First Book" || items[0]["status"] != "queued" {
		t.Fatalf("the paused book is back at the top, still on the Nightstand: %v", items)
	}
	if res, _ := admin.do("POST", "/api/queue/99999/pause", map[string]any{}, true); res.StatusCode != 404 {
		t.Fatalf("an unknown item: %d", res.StatusCode)
	}
}
