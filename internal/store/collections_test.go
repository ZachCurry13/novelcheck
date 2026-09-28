package store_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// A kid limited to collections sees only the books in theirs (even ones a
// parent marked OK) and only those collections; ideas are for parents.
func TestCollectionsAndKidLimits(t *testing.T) {
	s := newStore(t)
	cal, _ := s.EnsureCatalog(store.CalibreCatalogName, "calibre")
	zero := 0
	add := func(title, desc string) int64 {
		id, _ := s.UpsertBook(title, "Author", "", desc)
		_ = s.AddCopy(cal, id, "/c/"+title+".epub", "epub", title)
		_ = s.SaveAnalysis(id, store.Analysis{SpiceLevel: &zero, ContentSource: store.SourceAI})
		return id
	}
	dragon, pig, other := add("Dragon Rider", "A boy and a dragon fly north."), add("The Christmas Pig", "A toy pig is lost."), add("Other Book", "Nothing seasonal.")
	_ = s.SetApproved(other, true, "parent")

	summer, _ := s.CreateCollection(store.NewCollection{Name: "Summer reading", By: "parent"})
	idea, _ := s.CreateCollection(store.NewCollection{Name: "Dragons", Kind: "idea", Theme: "dragons", By: "AI"})
	if n, _ := s.AddToCollection(summer, []int64{dragon, dragon}, nil); n != 1 {
		t.Fatalf("added %d", n)
	}
	_, _ = s.AddToCollection(idea, []int64{dragon}, map[int64]string{dragon: "has a dragon"})

	kid, _ := s.CreateUserAge("kid", "password1", store.RoleRestricted, 0)
	kid.MaxSpice, kid.OnlyCollections, kid.Collections = -1, true, []int64{summer, idea} // the idea can't be given
	if err := s.UpdateUserProfile(kid); err != nil {
		t.Fatal(err)
	}
	kid, _ = s.UserByID(kid.ID)
	if !kid.OnlyCollections || len(kid.Collections) != 1 || kid.Collections[0] != summer {
		t.Fatalf("kid limits: %+v", kid)
	}
	bs, total, _ := s.ListBooks(store.BookFilter{}, kid)
	if total != 1 || bs[0].ID != dragon {
		t.Fatalf("kid sees %d books: %v", total, titles(bs))
	}
	if _, err := s.BookByID(other, kid); err == nil {
		t.Fatal("a book marked OK slipped past the collection limit")
	}
	cs, _ := s.ListCollections(kid)
	if len(cs) != 1 || cs[0].ID != summer || cs[0].Books != 1 || len(cs[0].Covers) != 1 {
		t.Fatalf("kid's collections: %+v", cs)
	}
	if all, _ := s.ListCollections(nil); len(all) != 2 || all[1].Kind != "idea" {
		t.Fatalf("parent's collections: %+v", all)
	}
	if bc := s.BookCollections(dragon, kid); len(bc) != 1 {
		t.Fatalf("book's collections for the kid: %+v", bc)
	}

	// Seasonal shelves: words, unless a parent built the shelf's collection.
	if bs, _, _ := s.ListBooks(store.BookFilter{Season: "christmas"}, nil); len(bs) != 1 || bs[0].ID != pig {
		t.Fatalf("christmas by words: %v", titles(bs))
	}
	if s.SeasonCollection("christmas") != 0 {
		t.Fatal("no Christmas collection yet")
	}
	xmas, _ := s.CreateCollection(store.NewCollection{Name: "Christmas", Season: "christmas", Kind: "ai"})
	_, _ = s.AddToCollection(xmas, []int64{other}, nil)
	if s.SeasonCollection("christmas") != xmas {
		t.Fatal("the built Christmas collection")
	}
	_ = s.KeepIdea(idea)
	if c, _ := s.CollectionByID(idea, nil); c.Kind != "ai" {
		t.Fatalf("kept idea: %+v", c)
	}
	_ = s.DeleteCollection(summer)
	if bs, total, _ = s.ListBooks(store.BookFilter{}, kid); total != 0 {
		t.Fatalf("collection deleted: kid sees %v", titles(bs))
	}
}
