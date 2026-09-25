package api_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestSuggestedReads(t *testing.T) {
	var pickID atomic.Int64 // the book the fake AI picks
	var aiCalls atomic.Int32
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		answer := `{"spice_level": 1, "spice_reason": "Light", "summary_verdict": "Fine."}`
		if r.URL.Path == "/chat/completions" {
			raw, _ := io.ReadAll(r.Body)
			if strings.Contains(string(raw), "You recommend books") {
				aiCalls.Add(1)
				answer = fmt.Sprintf(`{"picks": [{"id": %d, "reason": "Witty fantasy like Guards! Guards!"}],
					"outside": [{"title": "The Colour of Magic", "author": "Terry Pratchett", "reason": "Where Discworld began"}]}`, pickID.Load())
			}
		} else if r.URL.Path != "/search.json" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"docs": []any{},
			"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": answer}}},
			"usage":   map[string]int{"prompt_tokens": 100, "completion_tokens": 20}})
	}))
	defer fake.Close()
	srv, st := setupWith(t, func(s *api.Server) {
		s.Worker.NewEnricher = func() *enrich.Client {
			c := enrich.New("")
			c.OpenLibraryURL, c.GoogleBooksURL = fake.URL, fake.URL
			return c
		}
	})
	_ = st.SetSetting(store.KeyLLMBaseURL, fake.URL)
	_ = st.SetSetting(store.KeyLLMModel, "small-model")
	admin := login(t, srv, "admin", "adminpass1")
	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	ids := map[string]int64{}
	for i, b := range []struct{ title, desc, class string }{
		{"Guards! Guards! (Discworld, #8)", "The night watch of Ankh-Morpork faces a dragon summoned by a secret society.", "No Spice"},
		{"Men at Arms (Discworld, #9)", "The city watch hunts a killer with a strange new weapon.", "No Spice"},
		{"Small Gods (Discworld, #13)", "A novice monk and a tortoise-shaped god question a tyrannical church.", "No Spice"},
		{"Steamy Pratchett Parody", "A bawdy parody of the watch with explicit scenes.", "Open Door"},
	} {
		id, _ := st.UpsertBook(b.title, "Terry Pratchett", "", b.desc)
		_ = st.AddCopy(cat, id, fmt.Sprintf("/calibre/%d.epub", i), "epub", fmt.Sprint(i+1))
		_ = st.SaveAnalysis(id, store.Analysis{Classification: b.class})
		ids[b.title] = id
	}
	guards, menAtArms := ids["Guards! Guards! (Discworld, #8)"], ids["Men at Arms (Discworld, #9)"]
	pickID.Store(ids["Small Gods (Discworld, #13)"])
	get := func(c *client) map[string]any {
		t.Helper()
		res, out := c.do("GET", "/api/suggestions", nil, false)
		if res.StatusCode != 200 {
			t.Fatalf("suggestions: %d %v", res.StatusCode, out)
		}
		return out
	}
	first := func(out map[string]any) map[string]any { return out["items"].([]any)[0].(map[string]any) }

	// Free matching: the next book in the series, no AI calls.
	_ = st.SetSetting(store.KeySuggestMode, store.SuggestFree)
	if out := get(admin); len(out["items"].([]any)) != 0 {
		t.Fatalf("nothing to go on yet: %v", out)
	}
	admin.do("POST", "/api/queue", map[string]int64{"book_id": guards}, true)
	out := get(admin)
	if it := first(out); it["book"].(map[string]any)["title"] != "Men at Arms" || it["reason"] != "Next in Discworld after Guards! Guards!" || out["mode"] != "free" || aiCalls.Load() != 0 {
		t.Fatalf("next in series: %v", out)
	}

	// 👎 hides it for good.
	admin.do("POST", "/api/suggestions/vote", map[string]any{"book_id": menAtArms, "vote": -1}, true)
	// "Why not?" answers are checked, then saved with the 👎.
	if res, _ := admin.do("POST", "/api/suggestions/vote", map[string]any{"book_id": menAtArms, "vote": -1, "reason": "boring"}, true); res.StatusCode != 400 {
		t.Fatal("unknown reason")
	}
	if res, _ := admin.do("POST", "/api/suggestions/vote", map[string]any{"book_id": menAtArms, "vote": -1, "reason": "read"}, true); res.StatusCode != 200 {
		t.Fatal("already read it")
	}
	for _, it := range get(admin)["items"].([]any) {
		if it.(map[string]any)["book"].(map[string]any)["id"].(float64) == float64(menAtArms) {
			t.Fatal("a 👎 book comes back")
		}
	}

	// AI picks + books the family doesn't own, made in the background.
	_ = st.SetSetting(store.KeySuggestMode, store.SuggestAIOutside)
	if out := get(admin); out["refreshing"] != true {
		t.Fatalf("AI starts picking: %v", out)
	}
	deadline := time.Now().Add(5 * time.Second)
	for st.SuggestionSetFor(1) == nil && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	out = get(admin)
	if it := first(out); it["by_ai"] != true || it["reason"] != "Witty fantasy like Guards! Guards!" || out["refreshing"] != false || out["down"].(float64) != 1 {
		t.Fatalf("AI pick first: %v", out)
	}
	outside := out["outside"].([]any)
	if len(outside) != 1 || outside[0].(map[string]any)["title"] != "The Colour of Magic" || aiCalls.Load() != 1 {
		t.Fatalf("book the family doesn't own: %v", out)
	}
	get(admin)
	if aiCalls.Load() != 1 {
		t.Fatal("at most once a day")
	}

	// Wishing for it looks it up and takes it off the outside list.
	res, w := admin.do("POST", "/api/suggestions/wish", map[string]string{"title": "The Colour of Magic", "author": "Terry Pratchett", "reason": "Where Discworld began"}, true)
	if res.StatusCode != 200 || w["book_id"] == nil || len(get(admin)["outside"].([]any)) != 0 {
		t.Fatalf("wish: %d %v", res.StatusCode, w)
	}
	if _, wl := admin.do("GET", "/api/wishlist", nil, false); len(wl["items"].([]any)) != 1 {
		t.Fatalf("on the wishlist: %v", wl)
	}

	// Kids only get books they may see, never books from outside the library.
	admin.do("POST", "/api/admin/users", map[string]string{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	kid.do("POST", "/api/queue", map[string]int64{"book_id": guards}, true)
	for _, it := range get(kid)["items"].([]any) {
		if it.(map[string]any)["book"].(map[string]any)["title"] == "Steamy Pratchett Parody" {
			t.Fatal("hidden book suggested to a kid")
		}
	}
	if res, _ := kid.do("POST", "/api/suggestions/wish", map[string]string{"title": "Anything"}, true); res.StatusCode != 403 {
		t.Fatalf("kid wishing outside: %d", res.StatusCode)
	}

	// The admin's switch and choice.
	if res, _ := admin.do("PUT", "/api/admin/settings", map[string]string{"suggest_mode": "Whatever"}, true); res.StatusCode != 400 {
		t.Fatal("unknown mode")
	}
	admin.do("PUT", "/api/admin/settings", map[string]string{"module_suggestions": "false"}, true)
	if res, _ := admin.do("GET", "/api/suggestions", nil, false); res.StatusCode != 403 {
		t.Fatalf("switched off: %d", res.StatusCode)
	}
}
