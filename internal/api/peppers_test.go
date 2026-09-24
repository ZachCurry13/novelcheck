package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestPepperAPI(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	old, _ := st.UpsertBook("Old Rating", "A", "", "")
	_ = st.SaveAnalysis(old, store.Analysis{Classification: "Closed Door", Model: "gpt"})
	hand, _ := st.UpsertBook("Hand Rated", "B", "", "")

	// A parent rates by peppers; the old label follows.
	res, b := admin.do("PUT", "/api/books/"+itoa(hand)+"/verdict", map[string]any{"spice_level": 2, "summary_verdict": "Sweet."}, true)
	if res.StatusCode != 200 || b["spice_level"].(float64) != 2 || b["classification"] != "Closed Door" {
		t.Fatalf("verdict: %d %v", res.StatusCode, b)
	}
	if res, _ := admin.do("PUT", "/api/books/"+itoa(hand)+"/verdict", map[string]any{"spice_level": 6}, true); res.StatusCode != 400 {
		t.Fatalf("6 peppers must be refused: %d", res.StatusCode)
	}
	if _, out := admin.do("GET", "/api/books?spice=2", nil, false); out["total"].(float64) != 1 {
		t.Fatalf("pepper filter: %v", out)
	}

	// Kid limit: set to 1 pepper, so the 2-pepper book disappears.
	_, kid := admin.do("POST", "/api/admin/users", map[string]any{"username": "kiddo", "password": "kidpass12"}, true)
	kid["max_spice"], kid["hide_unrated"] = 1, false
	if res, out := admin.do("PUT", "/api/admin/users/"+itoa(int64(kid["id"].(float64))), kid, true); res.StatusCode != 200 || out["max_spice"].(float64) != 1 {
		t.Fatalf("set max peppers: %d %v", res.StatusCode, out)
	}
	k := login(t, srv, "kiddo", "kidpass12")
	if res, _ := k.do("GET", "/api/books/"+itoa(hand), nil, false); res.StatusCode != 404 {
		t.Fatalf("2-pepper book must be hidden from a 1-pepper kid: %d", res.StatusCode)
	}

	// Re-rating only picks AI-rated books from before the pepper scale.
	_, s := admin.do("GET", "/api/admin/status", nil, false)
	if s["rerate_candidates"].(float64) != 1 {
		t.Fatalf("rerate candidates: %v", s["rerate_candidates"])
	}
	if res, out := admin.do("POST", "/api/admin/rerate", nil, true); res.StatusCode != 200 || out["queued"].(float64) != 1 {
		t.Fatalf("rerate: %d %v", res.StatusCode, out)
	}
	if b, _ := st.BookByID(old, nil); b.Status != "analyzed" {
		t.Fatalf("re-rating must keep the book visible, status %s", b.Status)
	}
}
