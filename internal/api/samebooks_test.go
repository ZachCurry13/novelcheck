package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// A book on someone's list before its e-book arrived (so no author to match
// on) joins the library copy by itself; ambiguous pairs wait for a person.
func TestSameBooksJoin(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	look, _ := st.EnsureCatalog(store.LookedUpCatalog, "custom")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")

	listed, _ := st.UpsertBook("Cinder", "", "", "") // wished for without an author
	_ = st.AddCopy(look, listed, "lookup:cinder", "list", "")
	admin.do("POST", "/api/queue", map[string]int64{"book_id": listed}, true)
	if _, out := admin.do("POST", "/api/books/"+itoa(listed)+"/wish", map[string]string{}, true); out == nil {
		t.Fatal("wish")
	}
	real, _ := st.UpsertBook("Cinder (The Lunar Chronicles #1)", "Meyer, Marissa", "", "")
	_ = st.AddCopy(cal, real, "/c/cinder.epub", "epub", "7")
	if listed == real {
		t.Fatal("the test needs two separate entries")
	}

	_, out := admin.do("GET", "/api/admin/same-books", nil, false)
	if pairs, _ := out["pairs"].([]any); len(pairs) != 1 {
		t.Fatalf("one pair is offered: %v", out)
	}
	st.TidySame()
	_, out = admin.do("GET", "/api/admin/same-books", nil, false)
	if pairs, _ := out["pairs"].([]any); len(pairs) != 0 {
		t.Fatalf("the clear case is joined: %v", out)
	}
	if _, err := st.BookByID(listed, nil); err == nil && listed != real {
		if _, err2 := st.BookByID(real, nil); err2 == nil {
			t.Fatal("one of the two entries is gone")
		}
	}
	if _, items := admin.doList("GET", "/api/queue"); len(items) != 1 {
		t.Fatalf("the Nightstand entry followed the book: %v", items)
	}

	// two unowned entries stay for a person; "not the same" hides them
	a, _ := st.UpsertBook("Dust", "", "", "")
	b, _ := st.UpsertBook("Dust", "Hugh Howey", "", "")
	_ = st.AddCopy(look, a, "lookup:a", "list", "")
	_ = st.AddCopy(look, b, "lookup:b", "list", "")
	st.TidySame()
	_, out = admin.do("GET", "/api/admin/same-books", nil, false)
	if pairs, _ := out["pairs"].([]any); len(pairs) != 1 {
		t.Fatalf("unowned pairs wait for a person: %v", out)
	}
	if res, _ := admin.do("POST", "/api/admin/same-books/not", map[string]int64{"a": a, "b": b}, true); res.StatusCode != 200 {
		t.Fatal("not the same")
	}
	_, out = admin.do("GET", "/api/admin/same-books", nil, false)
	if pairs, _ := out["pairs"].([]any); len(pairs) != 0 {
		t.Fatalf("hidden for good: %v", out)
	}
}
