package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestStuffYourKindleEvent(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	owned, _ := st.UpsertBook("The Hobbit", "J.R.R. Tolkien", "", "")
	_ = st.AddCopy(cal, owned, "/c/hobbit.epub", "epub", "1")
	zero := 0
	_ = st.SaveAnalysis(owned, store.Analysis{SpiceLevel: &zero, ContentSource: store.SourceAI})

	res, out := admin.do("POST", "/api/events/preview", map[string]string{
		"html": `<a href="https://www.amazon.com/dp/B000000001">Free Book One by Ann Author</a><a href="https://www.amazon.com/dp/B000000002">The Hobbit by J.R.R. Tolkien</a>`}, true)
	books, _ := out["books"].([]any)
	if res.StatusCode != 200 || len(books) != 2 {
		t.Fatalf("preview: %d %v", res.StatusCode, out)
	}
	res, out = admin.do("POST", "/api/events", map[string]any{"name": "Stuff Your Kindle: Fall", "books": books}, true)
	if res.StatusCode != 201 || out["rating"].(float64) != 1 {
		t.Fatalf("create: %d %v", res.StatusCode, out)
	}
	ev := itoa(int64(out["id"].(float64)))
	_, out = admin.do("GET", "/api/events/"+ev, nil, false)
	list, _ := out["books"].([]any)
	if len(list) != 2 {
		t.Fatalf("event books: %v", out)
	}
	first, second := list[0].(map[string]any), list[1].(map[string]any)
	if first["title"] != "Free Book One" || first["asin"] != "B000000001" || first["owned"] != false || second["owned"] != true {
		t.Fatalf("marks: %v / %v", first, second)
	}
	fresh := int64(first["id"].(float64))
	if bs, total, _ := st.ListBooks(store.BookFilter{}, nil); total != 1 || bs[0].ID != owned {
		t.Fatalf("an event book slipped into the Library: %d", total)
	}

	// Kids only see rated event books.
	admin.do("POST", "/api/admin/users", map[string]string{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	if _, out = kid.do("GET", "/api/events/"+ev, nil, false); len(out["books"].([]any)) != 1 {
		t.Fatalf("kid sees unrated books: %v", out["books"])
	}
	if res, _ := kid.do("POST", "/api/events", map[string]any{"name": "x", "books": books}, true); res.StatusCode != 403 {
		t.Fatalf("kid made an event: %d", res.StatusCode)
	}

	// "✓ I claimed it" puts it in a library of the family's.
	kindle, _ := st.EnsureCatalog("Kindle (Amazon)", "drive")
	if res, _ := admin.do("POST", "/api/events/"+ev+"/books/"+itoa(fresh)+"/claim", map[string]int64{"catalog_id": cal}, true); res.StatusCode != 400 {
		t.Fatalf("claimed into Calibre: %d", res.StatusCode)
	}
	if res, out := admin.do("POST", "/api/events/"+ev+"/books/"+itoa(fresh)+"/claim", map[string]int64{"catalog_id": kindle}, true); res.StatusCode != 200 || out["owned"] != true {
		t.Fatalf("claim: %d %v", res.StatusCode, out)
	}
	if _, total, _ := st.ListBooks(store.BookFilter{}, nil); total != 2 {
		t.Fatalf("the claimed book isn't in the Library: %d", total)
	}

	// Pinned events stay; others go after 30 days, and so do their list copies.
	_, _ = st.DB.Exec(`UPDATE events SET created_at = datetime('now', '-31 days')`)
	admin.do("POST", "/api/events/"+ev+"/pin", map[string]bool{"pinned": true}, true)
	if n, _ := st.CleanEvents(); n != 0 {
		t.Fatal("a pinned event was cleaned")
	}
	admin.do("POST", "/api/events/"+ev+"/pin", map[string]bool{"pinned": false}, true)
	if n, _ := st.CleanEvents(); n != 1 {
		t.Fatal("old event kept")
	}
	if !st.Owned(fresh) {
		t.Fatal("cleaning dropped a claimed book")
	}
}
