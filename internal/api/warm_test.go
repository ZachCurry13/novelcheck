package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The Library cleanup page stays quick: its answer is kept, a request the app makes by itself whenever a page opens
// (marking notices read) does not throw it away, a background change to the library still shows the kept answer at
// once (it is worked out again behind it), and a change made through the app does throw it away.
func TestCleanupAnswerStaysKept(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	look := func() string {
		res, _ := admin.do("GET", "/api/admin/cleanup", nil, false)
		return res.Header.Get("X-Cache")
	}
	if got := look(); got != "miss" {
		t.Fatalf("the first look works it out: %q", got)
	}
	if got := look(); got != "hit" {
		t.Fatalf("then it is kept: %q", got)
	}
	if res, _ := admin.do("POST", "/api/notifications/read", map[string]any{}, true); res.StatusCode >= 400 {
		t.Fatalf("marking notices read: %d", res.StatusCode)
	}
	if got := look(); got != "hit" {
		t.Fatalf("what the app does by itself when a page opens is not a change: %q", got)
	}
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	id, _ := st.UpsertBook("A New Book", "Someone", "", "")
	_ = st.AddCalibreCopy(cal, id, "/c/new.epub", "epub", "1", "2026-01-01 10:00:00")
	if got := look(); got != "hit" {
		t.Fatalf("a book added by a background pass: the kept answer is shown at once: %q", got)
	}
	if res, _ := admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true); res.StatusCode != 201 {
		t.Fatalf("a real change: %d", res.StatusCode)
	}
	if got := look(); got != "miss" {
		t.Fatalf("a change made through the app throws the kept answer away: %q", got)
	}
}
