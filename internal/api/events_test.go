package api_test

import (
	"strconv"
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

	// The Library shows an event's books, with every filter and sort.
	_, out = admin.do("GET", "/api/books?event="+ev+"&sort=list", nil, false)
	if lb, _ := out["books"].([]any); len(lb) != 2 || lb[0].(map[string]any)["title"] != "Free Book One" {
		t.Fatalf("event in the Library: %v", out)
	}
	if _, out = admin.do("GET", "/api/books?event="+ev+"&not_owned=1&spice=0", nil, false); out["total"].(float64) != 0 {
		t.Fatalf("filters on an event: %v", out)
	}
	if _, out = kid.do("GET", "/api/books?event="+ev, nil, false); out["total"].(float64) != 1 {
		t.Fatalf("kid sees unrated event books in the Library: %v", out)
	}

	// Pinned events stay; others are archived after 30 days, not deleted.
	_, _ = st.DB.Exec(`UPDATE events SET created_at = datetime('now', '-31 days')`)
	admin.do("POST", "/api/events/"+ev+"/pin", map[string]bool{"pinned": true}, true)
	if n, _ := st.ArchiveEnded(); n != 0 {
		t.Fatal("a pinned event was archived")
	}
	admin.do("POST", "/api/events/"+ev+"/pin", map[string]bool{"pinned": false}, true)
	if n, _ := st.ArchiveEnded(); n != 1 {
		t.Fatal("old event not archived")
	}
	if e, _ := st.EventByID(int64(out2id(ev)), nil); e == nil || e.ArchivedAt == "" || e.Books != 2 {
		t.Fatalf("archived event: %+v", e)
	}
	// Restoring it keeps it (pinned, as it's old); an end that passes archives it.
	if res, _ := admin.do("POST", "/api/events/"+ev+"/archive", map[string]bool{"archived": false}, true); res.StatusCode != 200 {
		t.Fatalf("restore: %d", res.StatusCode)
	}
	if e, _ := st.EventByID(int64(out2id(ev)), nil); e.ArchivedAt != "" || !e.Pinned {
		t.Fatalf("restored: %+v", e)
	}
	if res, _ := admin.do("PATCH", "/api/events/"+ev, map[string]string{"name": "Fall event", "ends_at": "2020-01-01T05:00:00.000Z"}, true); res.StatusCode != 200 {
		t.Fatalf("edit: %d", res.StatusCode)
	}
	if e, _ := st.EventByID(int64(out2id(ev)), nil); e.ArchivedAt == "" || e.Name != "Fall event" || e.EndsAt != "2020-01-01T05:00:00Z" {
		t.Fatalf("ended: %+v", e)
	}
	if res, _ := admin.do("PATCH", "/api/events/"+ev, map[string]string{"name": "x", "ends_at": "soon"}, true); res.StatusCode != 400 {
		t.Fatalf("bad end: %d", res.StatusCode)
	}

	// Deleting is still there; claimed books stay.
	if res, _ := admin.do("DELETE", "/api/events/"+ev, nil, true); res.StatusCode != 200 {
		t.Fatalf("delete: %d", res.StatusCode)
	}
	if !st.Owned(fresh) {
		t.Fatal("deleting dropped a claimed book")
	}
}

func out2id(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
