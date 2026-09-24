package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestRulesVersionAndRerateAll(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	two := 2
	cur, _ := st.UpsertBook("Current Rules", "A", "", "")
	_ = st.SaveAnalysis(cur, store.Analysis{SpiceLevel: &two, SpiceReason: "Kissing only", Model: "gpt"})
	old, _ := st.UpsertBook("Older Rules", "B", "", "")
	_ = st.SaveAnalysis(old, store.Analysis{SpiceLevel: &two, Model: "gpt"})
	st.DB.MustExec(`UPDATE books SET rules_version = 1 WHERE id = ?`, old) // rated before the v1.16 wording
	hand, _ := st.UpsertBook("Hand Rated", "C", "", "")
	_ = st.SaveAnalysis(hand, store.Analysis{SpiceLevel: &two, Model: "manual: mom"})

	_, s := admin.do("GET", "/api/admin/status", nil, false)
	if s["rerate_candidates"].(float64) != 1 || s["ai_rated"].(float64) != 2 {
		t.Fatalf("older rules %v, AI-rated %v", s["rerate_candidates"], s["ai_rated"])
	}
	if _, b := admin.do("GET", "/api/books/"+itoa(cur), nil, false); b["book"].(map[string]any)["spice_reason"] != "Kissing only" {
		t.Fatalf("spice_reason missing: %v", b["book"])
	}
	if res, out := admin.do("POST", "/api/admin/rerate", map[string]string{"which": "all"}, true); res.StatusCode != 200 || out["queued"].(float64) != 2 {
		t.Fatalf("re-rate whole library must skip hand ratings: %d %v", res.StatusCode, out)
	}
	if res, _ := admin.do("POST", "/api/admin/rerate", map[string]string{"which": "everything"}, true); res.StatusCode != 400 {
		t.Fatalf("unknown re-rate choice: %d", res.StatusCode)
	}
}

func TestCalibreWebAddress(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	for in, want := range map[string]string{
		"192.168.1.10:8083":             "http://192.168.1.10:8083",
		"https://books.example.com/cw/": "https://books.example.com/cw",
		"":                              "",
	} {
		if res, _ := admin.do("PUT", "/api/admin/settings", map[string]string{"calibre_web_url": in}, true); res.StatusCode != 200 || st.Setting(store.KeyCalibreWebURL) != want {
			t.Fatalf("%q: %d, saved %q", in, res.StatusCode, st.Setting(store.KeyCalibreWebURL))
		}
	}
	for _, bad := range []string{"ftp://nas/books", "http://", "http://nas:8083/?x=1"} {
		if res, _ := admin.do("PUT", "/api/admin/settings", map[string]string{"calibre_web_url": bad}, true); res.StatusCode != 400 {
			t.Fatalf("%q must be refused: %d", bad, res.StatusCode)
		}
	}
	admin.do("PUT", "/api/admin/settings", map[string]string{"calibre_web_url": "nas:8083"}, true)

	zero := 0
	id, _ := st.UpsertBook("Clean Book", "D", "", "")
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &zero, Model: "gpt"})
	if _, b := admin.do("GET", "/api/books/"+itoa(id), nil, false); b["calibre_web_url"] != "http://nas:8083" {
		t.Fatalf("admins get the Calibre-Web address: %v", b["calibre_web_url"])
	}
	_, kid := admin.do("POST", "/api/admin/users", map[string]any{"username": "kiddo", "password": "kidpass12"}, true)
	kid["hide_unrated"] = false
	admin.do("PUT", "/api/admin/users/"+itoa(int64(kid["id"].(float64))), kid, true)
	k := login(t, srv, "kiddo", "kidpass12")
	res, b := k.do("GET", "/api/books/"+itoa(id), nil, false)
	if _, leaked := b["calibre_web_url"]; res.StatusCode != 200 || leaked {
		t.Fatalf("kids don't get the address: %d %v", res.StatusCode, b)
	}
}
