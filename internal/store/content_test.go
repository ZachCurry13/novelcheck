package store_test

import (
	"slices"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/content"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestContentItemsSavedAndShown(t *testing.T) {
	s := newStore(t)
	id, _ := s.UpsertBook("The Hunger Games", "Suzanne Collins", "", "")
	two := 2
	_ = s.SaveAnalysis(id, store.Analysis{SpiceLevel: &two, Model: "deep: m", Content: []string{"war", "murder", "bogus"},
		ContentSource: store.SourceDeep, ContentAmounts: map[string]int{"violence": content.ALot, "gore": content.ALittle}})
	b, _ := s.BookByID(id, nil)
	if b.Content != "murder:deep,war:deep" && b.Content != "war:deep,murder:deep" || b.ContentAmounts != "gore:1,violence:3" || !store.ContentChecked(b) {
		t.Fatalf("saved: %q %q %d", b.Content, b.ContentAmounts, b.ContentVersion)
	}
	// A parent's correction replaces the items but keeps the Deep Scan amounts.
	_ = s.SaveAnalysis(id, store.Analysis{SpiceLevel: &two, Model: "manual: mom", Content: []string{"war"}, ContentSource: store.SourceParent})
	if b, _ = s.BookByID(id, nil); b.Content != "war:parent" || b.ContentAmounts != "gore:1,violence:3" {
		t.Fatalf("parent: %q %q", b.Content, b.ContentAmounts)
	}
	// A rating that didn't check content leaves the items alone.
	_ = s.SaveAnalysis(id, store.Analysis{SpiceLevel: &two, Model: "manual: mom"})
	if b, _ = s.BookByID(id, nil); b.Content != "war:parent" {
		t.Fatalf("unchecked rating wiped items: %q", b.Content)
	}
}

func TestKidContentRules(t *testing.T) {
	s := newStore(t)
	zero := 0
	rate := func(title string, items ...string) int64 {
		id, _ := s.UpsertBook(title, "A", "", "")
		_ = s.SaveAnalysis(id, store.Analysis{SpiceLevel: &zero, Content: items, ContentSource: store.SourceAI})
		return id
	}
	war := rate("War Story", "war", "weapons")
	quiet := rate("Quiet Story")
	flagged := rate("Sweary Story")
	unchecked, _ := s.UpsertBook("Old Rating", "A", "", "")
	_ = s.SaveAnalysis(unchecked, store.Analysis{SpiceLevel: &zero})
	flag, _ := s.AddCustomFlag("Swearing", "")
	_ = s.SaveAnalysis(flagged, store.Analysis{SpiceLevel: &zero, CustomFlags: []string{flag.Key}, ContentSource: store.SourceAI})

	kid, _ := s.CreateUserAge("kid", "x", store.RoleRestricted, 5) // adults: no starter rules
	kid.HiddenContent = []string{"g:violence", "flag:" + flag.Key}
	if err := s.UpdateUserProfile(kid); err != nil {
		t.Fatal(err)
	}
	kid, _ = s.UserByID(kid.ID)
	seen := func() map[string]bool {
		bs, _, _ := s.ListBooks(store.BookFilter{}, kid)
		return titles(bs)
	}
	if got := seen(); got["War Story"] || got["Sweary Story"] || !got["Quiet Story"] || !got["Old Rating"] {
		t.Fatalf("group and custom rules: %v", got)
	}
	// With "Hide unrated", books not checked for content items wait, unless a
	// parent set their age group.
	kid.HideUnrated = true
	_ = s.UpdateUserProfile(kid)
	if got := seen(); got["Old Rating"] || !got["Quiet Story"] {
		t.Fatalf("unchecked with hide unrated: %v", got)
	}
	_ = s.SetBookAge(unchecked, 3, "mom")
	if got := seen(); !got["Old Rating"] {
		t.Fatalf("a parent's age group counts as checked: %v", got)
	}
	_ = s.SetBookAge(unchecked, 0, "")
	// LGBTQ+ was part of every rating before v1.19, so it alone doesn't hide unchecked books.
	kid.HiddenContent = []string{"lgbtq"}
	_ = s.UpdateUserProfile(kid)
	if got := seen(); !got["Old Rating"] || !got["War Story"] {
		t.Fatalf("lgbtq-only rule: %v", got)
	}
	// A parent's OK beats content rules; deleting a custom filter drops its rule.
	kid.HiddenContent = []string{"war", "flag:" + flag.Key}
	_ = s.UpdateUserProfile(kid)
	_ = s.SetApproved(war, true, "mom")
	if got := seen(); !got["War Story"] {
		t.Fatalf("approved book: %v", got)
	}
	_ = s.DeleteCustomFlag(flag.ID)
	if kid, _ = s.UserByID(kid.ID); !slices.Equal(kid.HiddenContent, []string{"war"}) {
		t.Fatalf("rules after deleting the filter: %v", kid.HiddenContent)
	}
	_ = quiet
}

func TestLibraryHidesContent(t *testing.T) {
	s := newStore(t)
	zero := 0
	for title, items := range map[string][]string{"Gory": {"blood", "corpses"}, "Murder Mystery": {"murder"}, "Picnic": nil} {
		id, _ := s.UpsertBook(title, "A", "", "")
		_ = s.SaveAnalysis(id, store.Analysis{SpiceLevel: &zero, Content: items, ContentSource: store.SourceAI})
	}
	bs, _, _ := s.ListBooks(store.BookFilter{ExcludeFlags: []string{"g:gore", "murder"}}, nil)
	if got := titles(bs); len(got) != 1 || !got["Picnic"] {
		t.Fatalf("hide gore and murder: %v", got)
	}
	bs, _, _ = s.ListBooks(store.BookFilter{Flags: []string{"g:gore"}}, nil)
	if got := titles(bs); len(got) != 1 || !got["Gory"] {
		t.Fatalf("has gore: %v", got)
	}
}

func TestStarterRulesAndContentChecks(t *testing.T) {
	s := newStore(t)
	young, _ := s.CreateUserAge("young", "x", store.RoleRestricted, 2)
	adult, _ := s.CreateUserAge("adult", "x", store.RoleRestricted, 5)
	parent, _ := s.CreateUserAge("parent", "x", store.RoleEditor, 0)
	if len(young.HiddenContent) != len(content.YoungPreset) || len(adult.HiddenContent) != 0 || len(parent.HiddenContent) != 0 {
		t.Fatalf("starter rules: %d %d %d", len(young.HiddenContent), len(adult.HiddenContent), len(parent.HiddenContent))
	}
	// Deep Scanned and hand-rated books get a content-only check, once.
	four := 4
	deep, _ := s.UpsertBook("Deep One", "A", "", "")
	_ = s.SaveAnalysis(deep, store.Analysis{SpiceLevel: &four, Model: store.DeepModelPrefix + "m", ContentSource: store.SourceAI})
	hand, _ := s.UpsertBook("Hand One", "A", "", "")
	_ = s.SaveAnalysis(hand, store.Analysis{SpiceLevel: &four, Model: "manual: mom"})
	ids, _ := s.RerateCandidates()
	if !slices.Equal(ids, []int64{hand}) {
		t.Fatalf("candidates %v, want only the unchecked hand-rated book %d", ids, hand)
	}
	b, _ := s.BookByID(hand, nil)
	if !store.KeepsRating(b) || store.ContentChecked(b) {
		t.Fatalf("hand-rated book: %+v", b)
	}
	_ = s.SaveContent(hand, []string{"alcohol"}, store.SourceAI, "")
	if ids, _ = s.RerateCandidates(); len(ids) != 0 {
		t.Fatalf("after the content check: %v", ids)
	}
	if b, _ = s.BookByID(hand, nil); *b.SpiceLevel != 4 || b.Content != "alcohol:ai" {
		t.Fatalf("content check changed the rating: %+v", b)
	}
}
