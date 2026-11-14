package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// A friend checks a book the family has: she gets its rating and can open
// its page and put it on her own Nightstand, and nothing says the family has
// it. ▶ on her Nightstand only marks it as reading, and the family's
// "Popular in the family" doesn't count her.
func TestAFriendChecksAndKeepsANightstand(t *testing.T) {
	fake := fakeBooks(t)
	srv, st := setupWith(t, func(s *api.Server) {
		s.Worker.NewEnricher = func() *enrich.Client {
			c := enrich.New("")
			c.OpenLibraryURL, c.GoogleBooksURL = fake.URL, fake.URL
			return c
		}
	})
	_ = st.SetSetting(store.KeyLLMBaseURL, fake.URL)
	_ = st.SetSetting(store.KeyLLMModel, "vision-model")
	admin := login(t, srv, "admin", "adminpass1")
	if res, _ := admin.do("POST", "/api/admin/users", map[string]string{"username": "pal", "password": "palpass123", "role": "friend"}, true); res.StatusCode != 201 {
		t.Fatalf("friend: %d", res.StatusCode)
	}
	cal, _ := st.EnsureCatalog("Calibre Main", "calibre")
	hobbit, _ := st.UpsertBook("The Hobbit", "J. R. R. Tolkien", "", "")
	_ = st.AddCopy(cal, hobbit, "/calibre/hobbit.epub", "epub", "7")
	zero := 0
	_ = st.SaveAnalysis(hobbit, store.Analysis{SpiceLevel: &zero, Model: "gpt"})
	pal := login(t, srv, "pal", "palpass123")

	path := sprintf("/api/books/%d", hobbit)
	if res, _ := pal.do("GET", path, nil, false); res.StatusCode == 200 {
		t.Fatal("the family's book opened before she checked it")
	}
	res, out := pal.do("POST", "/api/check", map[string]string{"query": "the hobbit tolkien"}, true)
	if res.StatusCode != 200 || out["in_library"] != false || len(out["catalogs"].([]any)) != 0 || out["friend"] != true ||
		out["book"].(map[string]any)["formats"] != "" || out["book"].(map[string]any)["id"].(float64) != float64(hobbit) {
		t.Fatalf("her check: %d %v", res.StatusCode, out)
	}
	if res, page := pal.do("GET", path, nil, false); res.StatusCode != 200 || len(page["copies"].([]any)) != 0 {
		t.Fatalf("the checked book's page: %d %v", res.StatusCode, page)
	}

	if res, _ := pal.do("POST", "/api/queue", map[string]int64{"book_id": hobbit}, true); res.StatusCode != 201 {
		t.Fatalf("her Nightstand: %d", res.StatusCode)
	}
	res, items := pal.doList("GET", "/api/queue")
	if res.StatusCode != 200 || len(items) != 1 || items[0]["owned"] != false || items[0]["file"] != "" && items[0]["file"] != nil {
		t.Fatalf("her pile: %v", items)
	}
	item := int64(items[0]["id"].(float64))
	if res, out := pal.do("POST", sprintf("/api/queue/%d/start", item), nil, true); res.StatusCode != 200 || out["delivery_note"] != "Marked as reading" {
		t.Fatalf("start reading: %d %v", res.StatusCode, out)
	}
	var told int
	_ = st.DB.Get(&told, `SELECT COUNT(*) FROM notifications WHERE message LIKE '%pal%'`)
	if told != 0 {
		t.Fatal("the family was told what she reads")
	}
	if fav, _ := st.FamilyFavorites(&store.User{ID: 1, Role: store.RoleAdmin}, 20); len(fav) != 0 {
		t.Fatalf("her pick counted as the family's: %v", fav)
	}
	// The parents' tools behind Check stay closed to her.
	if res, _ := pal.do("GET", "/api/activity", nil, false); res.StatusCode != 403 {
		t.Fatalf("activity: %d", res.StatusCode)
	}
	if res, _ := pal.do("GET", "/api/suggestions", nil, false); res.StatusCode != 403 {
		t.Fatalf("suggestions: %d", res.StatusCode)
	}
}
