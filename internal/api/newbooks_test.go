package api_test

import (
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/push"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// New e-books and audiobooks are announced to everyone, once: not on the first look, and not when switched off.
func TestNewBookNotices(t *testing.T) {
	var server *api.Server
	_, st := setupWith(t, func(s *api.Server) { server = s })
	got := 0
	var bodies []string
	send := func(id int64, m push.Message) { got++; bodies = append(bodies, m.Body) }
	server.AnnounceNewBooks(time.Now().Add(-time.Hour), send) // first look: only remembers the time
	if got != 0 {
		t.Fatalf("nothing is announced on the first look: %v", bodies)
	}
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	abs, _ := st.EnsureCatalog("Audiobooks", "custom")
	for _, title := range []string{"One", "Two"} {
		id, _ := st.UpsertBook(title, "A", "", "")
		_ = st.AddCopy(cal, id, "/c/"+title+".epub", "epub", "1")
	}
	id, _ := st.UpsertBook("Sound", "A", "", "")
	_ = st.AddCopy(abs, id, "abs:1", store.AudioFormat, "1")
	server.AnnounceNewBooks(time.Now(), send)
	users, _ := st.ListUsers()
	if got != 2*len(users) || bodies[0] != "2 new books" || bodies[1] != "1 new audiobook" {
		t.Fatalf("two notices per person: %d %v", got, bodies)
	}
	got = 0
	server.AnnounceNewBooks(time.Now().Add(time.Hour), send)
	if got != 0 {
		t.Fatal("the same books are not announced twice")
	}
}
