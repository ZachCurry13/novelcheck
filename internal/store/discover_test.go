package store_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestDiscoverBooksStayOutOfTheLibrary(t *testing.T) {
	s := newStore(t)
	cal, _ := s.EnsureCatalog(store.CalibreCatalogName, "calibre")
	owned, _ := s.UpsertBook("The Hobbit", "J.R.R. Tolkien", "", "")
	_ = s.AddCopy(cal, owned, "/c/hobbit.epub", "epub", "1")
	n, err := s.SaveDiscoverList("nyt:young-adult-hardcover", []store.DiscoverEntry{
		{Rank: 1, Title: "New Book", Author: "Ann Author", ISBN: "9780000000001", CoverURL: "https://storage.googleapis.com/a.jpg", WeeksOnList: 1},
		{Rank: 2, Title: "The Hobbit", Author: "J.R.R. Tolkien"},
		{Rank: 3, Title: "Racy Book", Author: "Bea Author"},
	})
	if err != nil || n != 3 {
		t.Fatalf("saved %d: %v", n, err)
	}
	newID := s.MatchBook("New Book", "Ann Author")
	racy := s.MatchBook("Racy Book", "Bea Author")
	five := 5
	_ = s.SaveAnalysis(racy, store.Analysis{SpiceLevel: &five, ContentSource: store.SourceAI})

	// Not the family's: not owned, not in the Library, not counted, not in "Analyze batch".
	bs, _, _ := s.ListBooks(store.BookFilter{}, nil)
	if got := titles(bs); len(got) != 1 || !got["The Hobbit"] || s.Owned(newID) || !s.Owned(owned) {
		t.Fatalf("library %v, owned new %v", got, s.Owned(newID))
	}
	if counts, _ := s.StatusCounts(); counts["pending"] != 1 { // only the Hobbit
		t.Fatalf("status counts %v", counts)
	}
	if ids, _ := s.QueueForAnalysis(10); len(ids) != 1 || ids[0] != owned {
		t.Fatalf("batch picked %v", ids)
	}
	cats, _ := s.ListCatalogs(nil)
	for _, c := range cats {
		if c.Name == store.DiscoverCatalog {
			t.Fatal("the Discover catalog is listed")
		}
	}
	// The owned book keeps its own library, without a Discover copy.
	if copies, _ := s.BookCopies(owned); len(copies) != 1 {
		t.Fatalf("owned book copies: %+v", copies)
	}

	// Parents see everything listed; a kid never sees unrated books, and the
	// kid's pepper limit still applies.
	all, _ := s.DiscoverItems(nil)
	kid, _ := s.CreateUserAge("kid", "x", store.RoleRestricted, 5)
	kid.MaxSpice = 2
	_ = s.UpdateUserProfile(kid)
	kid, _ = s.UserByID(kid.ID)
	forKid, _ := s.DiscoverItems(kid)
	if len(all) != 3 || len(forKid) != 0 {
		t.Fatalf("parent sees %d, kid sees %d", len(all), len(forKid))
	}
	if all[0].Title != "New Book" || all[0].Owned || !all[1].Owned || s.DiscoverCoverURL(newID) != "https://storage.googleapis.com/a.jpg" {
		t.Fatalf("items: %+v", all[0])
	}

	// Rating goes best-ranked first and only for unrated books.
	ids, _ := s.QueueDiscoverRatings(1)
	if len(ids) != 1 || ids[0] != newID {
		t.Fatalf("to rate: %v", ids)
	}
	if c := s.DiscoverCounts(); c.Listed != 3 || c.Rated != 1 || c.Waiting != 2 {
		t.Fatalf("counts %+v", c)
	}
}
