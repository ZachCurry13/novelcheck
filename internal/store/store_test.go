package store_test

import (
	"fmt"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	d, err := db.OpenDSN(fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=foreign_keys(1)", t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return store.New(d)
}

func TestNormKeyMatchesAcrossSources(t *testing.T) {
	a := store.NormKey("The Hobbit", "J.R.R. Tolkien")
	b := store.NormKey("the hobbit!", "Tolkien, J. R. R.")
	if a != b {
		t.Fatalf("keys differ: %q vs %q", a, b)
	}
}

func seed(t *testing.T, s *store.Store) (cal, kindle int64, ids map[string]int64) {
	t.Helper()
	cal, _ = s.EnsureCatalog(store.CalibreCatalogName, "calibre")
	kindle, _ = s.EnsureCatalog("Jenna's Kindle", "drive")
	ids = map[string]int64{}
	add := func(title string, cat int64, a *store.Analysis) {
		id, err := s.UpsertBook(title, "Author "+title, "", "")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.AddCopy(cat, id, "/p/"+title, "epub", title); err != nil {
			t.Fatal(err)
		}
		if a != nil {
			if err := s.SaveAnalysis(id, *a); err != nil {
				t.Fatal(err)
			}
		}
		ids[title] = id
	}
	add("Clean", cal, &store.Analysis{Classification: "No Spice"})
	add("Steamy", cal, &store.Analysis{Classification: "Open Door", Nudity: true})
	add("Spooky", kindle, &store.Analysis{Classification: "Closed Door", DarkOccult: true})
	add("Unrated", kindle, nil)
	// Same title on both devices -> overlap.
	id, _ := s.UpsertBook("Clean", "Author Clean", "", "")
	_ = s.AddCopy(kindle, id, "documents/Clean.azw3", "azw3", "B000")
	return
}

func titles(bs []store.Book) map[string]bool {
	m := map[string]bool{}
	for _, b := range bs {
		m[b.Title] = true
	}
	return m
}

func TestRestrictedVisibility(t *testing.T) {
	s := newStore(t)
	_, _, ids := seed(t, s)
	kid, err := s.CreateUser("kid", "x", store.RoleRestricted)
	if err != nil {
		t.Fatal(err)
	}
	books, total, err := s.ListBooks(store.BookFilter{}, kid)
	if err != nil {
		t.Fatal(err)
	}
	got := titles(books)
	if total != 1 || !got["Clean"] {
		t.Fatalf("restricted user should only see Clean, got %v", got)
	}
	if _, err := s.BookByID(ids["Steamy"], kid); err != store.ErrNotFound {
		t.Fatalf("restricted user fetched hidden book: %v", err)
	}
	// Search must not leak hidden titles either.
	books, _, _ = s.ListBooks(store.BookFilter{Query: "Steamy"}, kid)
	if len(books) != 0 {
		t.Fatal("search leaked a hidden title")
	}
}

func TestFilters(t *testing.T) {
	s := newStore(t)
	cal, kindle, _ := seed(t, s)
	cases := []struct {
		name string
		f    store.BookFilter
		want []string
	}{
		{"catalog", store.BookFilter{CatalogID: kindle}, []string{"Spooky", "Unrated", "Clean"}},
		{"overlap", store.BookFilter{CatalogID: cal, OverlapWith: kindle}, []string{"Clean"}},
		{"multi", store.BookFilter{MultiCatalog: true}, []string{"Clean"}},
		{"open door", store.BookFilter{Classification: "Open Door"}, []string{"Steamy"}},
		{"pending", store.BookFilter{Classification: "Pending"}, []string{"Unrated"}},
		{"flag", store.BookFilter{Flags: []string{"dark_occult"}}, []string{"Spooky"}},
		{"exclude", store.BookFilter{ExcludeFlags: []string{"nudity", "dark_occult"}}, []string{"Clean", "Unrated"}},
	}
	for _, c := range cases {
		books, total, err := s.ListBooks(c.f, nil)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got := titles(books)
		if total != len(c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
			continue
		}
		for _, w := range c.want {
			if !got[w] {
				t.Errorf("%s: missing %s in %v", c.name, w, got)
			}
		}
	}
}

func TestQueueOrdering(t *testing.T) {
	s := newStore(t)
	_, _, ids := seed(t, s)
	u, _ := s.CreateUser("reader", "x", store.RoleAdmin)
	for _, n := range []string{"Clean", "Steamy", "Spooky"} {
		if err := s.Enqueue(u.ID, ids[n]); err != nil {
			t.Fatal(err)
		}
	}
	items, _ := s.ListQueue(u.ID, false)
	if len(items) != 3 || items[0].Title != "Clean" {
		t.Fatalf("unexpected initial queue %+v", items)
	}
	if err := s.ReorderQueue(u.ID, []int64{items[2].ID, items[0].ID, items[1].ID}); err != nil {
		t.Fatal(err)
	}
	items, _ = s.ListQueue(u.ID, false)
	if items[0].Title != "Spooky" || items[1].Title != "Clean" {
		t.Fatalf("reorder not persisted: %+v", items)
	}
	if err := s.SetQueueStatus(u.ID, items[1].ID, "reading", "note"); err != nil {
		t.Fatal(err)
	}
	items, _ = s.ListQueue(u.ID, false)
	if items[0].Status != "reading" || items[0].Title != "Clean" {
		t.Fatalf("reading item should sort first: %+v", items)
	}
	// Another user cannot reorder someone else's items.
	other, _ := s.CreateUser("other", "x", store.RoleRestricted)
	if err := s.ReorderQueue(other.ID, []int64{items[0].ID}); err == nil {
		t.Fatal("expected error reordering another user's queue")
	}
}

func TestAnalysisQueueLifecycle(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	ids, err := s.QueueForAnalysis(10)
	if err != nil || len(ids) != 1 {
		t.Fatalf("expected the one pending book queued, got %v %v", ids, err)
	}
	n, _ := s.ResetQueued()
	if n != 1 {
		t.Fatalf("wipe reset %d, want 1", n)
	}
	counts, _ := s.StatusCounts()
	if counts["pending"] != 1 || counts["analyzed"] != 3 {
		t.Fatalf("unexpected counts %v", counts)
	}
}

// A parent's "OK" mark overrides kids' content rules and hide filters, and
// keeps the book off the Calibre removal list.
func TestApprovalOverridesFilters(t *testing.T) {
	s := newStore(t)
	_, _, ids := seed(t, s)
	kid, _ := s.CreateUser("kid", "x", store.RoleRestricted)

	if _, err := s.BookByID(ids["Spooky"], kid); err != store.ErrNotFound {
		t.Fatal("Spooky should be hidden before approval")
	}
	hide := store.BookFilter{ExcludeFlags: []string{"dark_occult"}}
	if books, _, _ := s.ListBooks(hide, nil); titles(books)["Spooky"] {
		t.Fatal("hide filter should hide Spooky")
	}
	matches, _ := s.CalibreMatches(store.BookFilter{AnyFlags: []string{"nudity", "open_door"}})
	if len(matches) != 1 || matches[0].Title != "Steamy" {
		t.Fatalf("removal list: %+v", matches)
	}

	if err := s.SetApproved(ids["Spooky"], true, "mom"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetApproved(ids["Steamy"], true, "mom"); err != nil {
		t.Fatal(err)
	}
	b, err := s.BookByID(ids["Spooky"], kid)
	if err != nil || !b.Approved || b.ApprovedBy != "mom" {
		t.Fatalf("approved book should be visible to kid: %+v %v", b, err)
	}
	if books, _, _ := s.ListBooks(hide, nil); !titles(books)["Spooky"] {
		t.Fatal("approved book should survive hide filter")
	}
	if matches, _ := s.CalibreMatches(store.BookFilter{AnyFlags: []string{"nudity"}}); len(matches) != 0 {
		t.Fatalf("approved book must not be offered for removal: %+v", matches)
	}
	_ = s.SetApproved(ids["Spooky"], false, "mom")
	if b, _ := s.BookByID(ids["Spooky"], nil); b.Approved || b.ApprovedBy != "" {
		t.Fatalf("approval not cleared: %+v", b)
	}
}
