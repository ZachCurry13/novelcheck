package api_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Each grown-up chooses Hide, Heads-up or Don't mention per label; a heads-up says where a Deep Scan found the spice.
func TestContentPrefs(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]any{"username": "sister", "password": "sisterpass1", "role": "editor"}, true)
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass123"}, true)
	sister := login(t, srv, "sister", "sisterpass1")
	cal, _ := st.EnsureCatalog("Family", "custom")
	four := 4
	mk := func(title string, items []string, nudity bool) int64 {
		id, _ := st.UpsertBook(title, "Writer", "", "")
		_ = st.AddCopy(cal, id, title, "list", "")
		_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &four, Classification: "Open Door", Nudity: nudity, Content: items, ContentSource: store.SourceAI})
		return id
	}
	spicy := mk("Spicy Book", []string{"lgbtq", "strong_profanity"}, false)
	bare := mk("Bare Book", nil, true)
	_, _ = st.DB.Exec(`INSERT INTO deep_reads (book_id, status, words, notes) VALUES (?, 'done', 1000, ?)`, spicy,
		`[{"label":"Chapter 12","level":4,"note":"x","from":420,"to":500}]`)

	if res, _ := sister.do("PUT", "/api/me/content-prefs", map[string]any{"hide": []string{"nudity"}, "warn": []string{"lgbtq", "g:language"},
		"hide_above": -1, "warn_from": 3}, true); res.StatusCode != 200 {
		t.Fatalf("save: %d", res.StatusCode)
	}
	if res, _ := sister.do("GET", "/api/books/"+itoa(bare), nil, false); res.StatusCode != 404 {
		t.Fatalf("Hide nudity hides the book for her: %d", res.StatusCode)
	}
	if res, _ := admin.do("GET", "/api/books/"+itoa(bare), nil, false); res.StatusCode != 200 {
		t.Fatalf("nobody else: %d", res.StatusCode)
	}
	_, page := sister.do("GET", "/api/books/"+itoa(spicy), nil, false)
	heads := fmt.Sprint(page["heads_up"])
	for _, want := range []string{"Spice level 4: around 42% in (Chapter 12)", "LGBTQ+", "Language"} {
		if !strings.Contains(heads, want) {
			t.Fatalf("heads-up %q missing: %s", want, heads)
		}
	}
	_, mine := sister.do("GET", "/api/me/content-prefs", nil, false)
	if p := fmt.Sprint(mine["prefs"]); !strings.Contains(p, "nudity") || !strings.Contains(p, "warn_from:3") {
		t.Fatalf("read back: %s", p)
	}
	kid := login(t, srv, "kid", "kidpass123")
	if res, _ := kid.do("PUT", "/api/me/content-prefs", map[string]any{"hide": []string{}, "warn": []string{}, "hide_above": -1, "warn_from": -1}, true); res.StatusCode != 403 {
		t.Fatalf("a kid can't change their own rules: %d", res.StatusCode)
	}
}
