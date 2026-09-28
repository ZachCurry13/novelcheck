package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Paper books: a parent makes a physical library, scans books into it (new
// ones are queued for rating), and a paper-only book in Up Next is marked
// as reading without anything to send.
func TestPaperBooks(t *testing.T) {
	fake := fakeBooks(t)
	srv, st := setupWith(t, func(s *api.Server) {
		s.Worker.NewEnricher = func() *enrich.Client {
			c := enrich.New("")
			c.OpenLibraryURL, c.GoogleBooksURL = fake.URL, fake.URL
			return c
		}
	})
	admin := login(t, srv, "admin", "adminpass1")

	res, out := admin.do("POST", "/api/shelves", map[string]string{"name": "Living room shelf"}, true)
	if res.StatusCode != 201 {
		t.Fatalf("create: %d %v", res.StatusCode, out)
	}
	shelf := int64(out["id"].(float64))
	// A library of e-books can't be turned into a shelf, nor imported into once it is one.
	_, _ = st.EnsureCatalog("Kindle", "drive")
	if res, _ := admin.do("POST", "/api/shelves", map[string]string{"name": "kindle"}, true); res.StatusCode != 400 {
		t.Fatalf("e-book library name: %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", "/api/import/drive", map[string]any{"catalog_id": shelf,
		"books": []map[string]string{{"title": "X", "format": "epub", "path": "x.epub"}}}, true); res.StatusCode != 400 {
		t.Fatalf("import into a shelf: %d", res.StatusCode)
	}

	// A book NovelCheck doesn't know: looked up, put on the shelf, queued first.
	res, out = admin.do("POST", "/api/shelves/"+itoa(shelf)+"/books", map[string]string{"query": "the hobbit"}, true)
	b, _ := out["book"].(map[string]any)
	if res.StatusCode != 200 || out["added"] != true || b["title"] != "The Hobbit" || b["status"] != "queued" {
		t.Fatalf("add: %d %v", res.StatusCode, out)
	}
	id := int64(b["id"].(float64))
	if res, out = admin.do("POST", "/api/shelves/"+itoa(shelf)+"/books", map[string]string{"query": "The Hobbit"}, true); out["added"] != false {
		t.Fatalf("second scan: %d %v", res.StatusCode, out)
	}
	if !st.Owned(id) || !st.PaperOnly(id) {
		t.Fatalf("owned %v, paper only %v", st.Owned(id), st.PaperOnly(id))
	}
	bs, total, _ := st.ListBooks(store.BookFilter{Format: "paper"}, nil)
	if total != 1 || bs[0].Formats != "PAPER" {
		t.Fatalf("paper filter: %d %+v", total, bs)
	}
	if _, total, _ := st.ListBooks(store.BookFilter{Format: "none"}, nil); total != 1 {
		t.Fatalf("a paper book has no file: %d", total)
	}

	// Start Reading doesn't try to email a printed book.
	u, _ := st.UserByName("admin")
	u.DeliveryMethod, u.KindleEmail = "email", "someone@kindle.com"
	_ = st.UpdateUserProfile(u)
	_, _ = admin.do("POST", "/api/queue", map[string]any{"book_id": id}, true)
	items, _ := st.ListQueue(u.ID, false)
	res, out = admin.do("POST", "/api/queue/"+itoa(items[0].ID)+"/start", nil, true)
	if res.StatusCode != 200 || out["status"] != "reading" {
		t.Fatalf("start a paper book: %d %v", res.StatusCode, out)
	}

	// Not a shelf: refused.
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	if res, _ := admin.do("POST", "/api/shelves/"+itoa(cal)+"/books", map[string]string{"query": "x"}, true); res.StatusCode != 404 {
		t.Fatalf("add to Calibre: %d", res.StatusCode)
	}
}
