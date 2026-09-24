package store_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

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

func TestNotifications(t *testing.T) {
	s := newStore(t)
	s.Notify("error", "tunnel", "Remote access disconnected", "#/admin")
	s.Notify("error", "tunnel", "Remote access disconnected", "#/admin")
	s.Notify("warning", "analysis", "Rating failed: bad key", "#/admin")
	items, unread, err := s.Notifications(50)
	if err != nil || unread != 2 || len(items) != 2 {
		t.Fatalf("got %d unread, %d items, %v", unread, len(items), err)
	}
	for _, n := range items {
		if n.Source == "tunnel" && n.Count != 2 {
			t.Fatalf("duplicate should bump count: %+v", n)
		}
	}
	s.Resolve("tunnel")
	if _, unread, _ = s.Notifications(50); unread != 1 {
		t.Fatalf("resolve should mark tunnel read, unread=%d", unread)
	}
	// A new occurrence after resolving is a fresh notification.
	s.Notify("error", "tunnel", "Remote access disconnected", "#/admin")
	if items, _, _ = s.Notifications(50); len(items) != 3 {
		t.Fatalf("expected a new row after resolve, got %d", len(items))
	}
	_ = s.MarkNotificationsRead(0)
	if _, unread, _ = s.Notifications(50); unread != 0 {
		t.Fatal("mark all read failed")
	}
	_ = s.ClearNotifications()
	if items, _, _ = s.Notifications(50); len(items) != 0 {
		t.Fatal("clear failed")
	}
}

// Kids see books rated for their age group or younger, plus books without an
// age group (which fall back to their content rules).
func TestAgeGroups(t *testing.T) {
	s := newStore(t)
	_, _, ids := seed(t, s)
	zero := 0
	_ = s.SaveAnalysis(ids["Clean"], store.Analysis{SpiceLevel: &zero}) // 0 peppers
	_ = s.SetBookAge(ids["Clean"], 1, "mom")                            // young kids
	_ = s.SetBookAge(ids["Spooky"], 3, "mom")                           // teens
	_ = s.SetApproved(ids["Spooky"], true, "mom")

	young, _ := s.CreateUserAge("little", "x", store.RoleRestricted, 1)
	teen, _ := s.CreateUserAge("teen", "x", store.RoleRestricted, 3)
	if !young.HideUnrated || young.AgeLevel != 1 || teen.AgeLevel != 3 {
		t.Fatalf("age presets not applied: %+v %+v", young, teen)
	}
	yb, _, _ := s.ListBooks(store.BookFilter{}, young)
	tb, _, _ := s.ListBooks(store.BookFilter{}, teen)
	if got := titles(yb); !got["Clean"] || got["Spooky"] {
		t.Fatalf("young kid should see Clean only (OK mark doesn't beat age group): %v", got)
	}
	if got := titles(tb); !got["Spooky"] || !got["Clean"] {
		t.Fatalf("teen should see Clean and the approved teen book: %v", got)
	}
	// A parent-rated age group counts as rated even before AI analysis.
	pending, _ := s.UpsertBook("Unrated Picture Book", "A", "", "")
	if _, err := s.BookByID(pending, young); err != store.ErrNotFound {
		t.Fatal("unrated book should be hidden from kids")
	}
	_ = s.SetBookAge(pending, 1, "mom")
	if _, err := s.BookByID(pending, young); err != nil {
		t.Fatal("parent-rated book should show for its age group:", err)
	}
	_ = s.SetBookAge(pending, 0, "mom")

	suitable, _, _ := s.ListBooks(store.BookFilter{Age: "2"}, nil)
	if got := titles(suitable); len(got) != 1 || !got["Clean"] {
		t.Fatalf("'suitable up to middle grade' filter: %v", got)
	}
	unset, _, _ := s.ListBooks(store.BookFilter{Age: "unset"}, nil)
	if titles(unset)["Clean"] {
		t.Fatal("unset filter returned an age-rated book")
	}
	if _, err := s.CreateUserAge("bad", "x", store.RoleRestricted, 9); err == nil {
		t.Fatal("invalid age accepted")
	}
}

func TestBookNotes(t *testing.T) {
	s := newStore(t)
	_, _, ids := seed(t, s)
	mom, _ := s.CreateUser("mom", "x", store.RoleEditor)
	kid, _ := s.CreateUser("kid", "x", store.RoleRestricted)
	if _, err := s.AddNote(ids["Clean"], mom.ID, "Loved it; fine for 10+.", "everyone"); err != nil {
		t.Fatal(err)
	}
	pid, _ := s.AddNote(ids["Clean"], mom.ID, "Chapter 12 has a scary scene.", "parents")
	if _, err := s.AddNote(ids["Clean"], mom.ID, "  ", "everyone"); err == nil {
		t.Fatal("empty note accepted")
	}
	all, _ := s.BookNotes(ids["Clean"], mom)
	kids, _ := s.BookNotes(ids["Clean"], kid)
	if len(all) != 2 || len(kids) != 1 || kids[0].Author != "mom" {
		t.Fatalf("visibility wrong: parents see %d, kid sees %+v", len(all), kids)
	}
	_ = s.UpdateNote(pid, "Chapter 12 is intense.", "everyone")
	if kids, _ = s.BookNotes(ids["Clean"], kid); len(kids) != 2 {
		t.Fatal("updated note should now be visible to kids")
	}
	_ = s.DeleteNote(pid)
	if all, _ = s.BookNotes(ids["Clean"], mom); len(all) != 1 {
		t.Fatal("delete failed")
	}
}

func TestDailyTokens(t *testing.T) {
	s := newStore(t)
	_, _, ids := seed(t, s)
	if err := s.RecordUsage(ids["Spooky"], "m", 100, 20); err != nil {
		t.Fatal(err)
	}
	days, err := s.DailyTokens(14)
	if err != nil || len(days) != 14 {
		t.Fatalf("days=%d err=%v", len(days), err)
	}
	if last := days[13]; last.Tokens != 120 || last.Calls != 1 {
		t.Fatalf("today = %+v", last)
	}
	if days[0].Tokens != 0 {
		t.Fatalf("quiet day should be zero: %+v", days[0])
	}
}

func TestSuggestKeep(t *testing.T) {
	e := func(id string, sizes map[string]int64) store.DupEntry {
		d := store.DupEntry{CalibreID: id}
		for f, sz := range sizes {
			d.Files = append(d.Files, store.DupFile{Format: f, Size: sz})
		}
		return d
	}
	cases := []struct {
		entries []store.DupEntry
		want    string
	}{
		{[]store.DupEntry{e("1", map[string]int64{"MOBI": 9}), e("2", map[string]int64{"EPUB": 1})}, "2"},
		{[]store.DupEntry{e("1", map[string]int64{"EPUB": 1}), e("2", map[string]int64{"EPUB": 1, "PDF": 1})}, "2"},
		{[]store.DupEntry{e("1", map[string]int64{"EPUB": 5}), e("2", map[string]int64{"EPUB": 9})}, "2"},
		{[]store.DupEntry{e("1", map[string]int64{"EPUB": 5}), e("2", map[string]int64{"EPUB": 5})}, "1"},
	}
	for i, c := range cases {
		if got := store.SuggestKeep(c.entries); got != c.want {
			t.Errorf("case %d: keep %s, want %s", i, got, c.want)
		}
	}
}

func TestLLMModelsOrder(t *testing.T) {
	s := newStore(t)
	_ = s.SetSetting(store.KeyLLMModel, "qwen2.5:7b")
	_ = s.SetSetting(store.KeyLLMFallbackModel, " llama3.1:8b , llama3.2,qwen2.5:7b,, ")
	got := s.LLMModels()
	if len(got) != 3 || got[0] != "qwen2.5:7b" || got[1] != "llama3.1:8b" || got[2] != "llama3.2" {
		t.Fatalf("models: %v", got)
	}
}

func TestPepperScale(t *testing.T) {
	s := newStore(t)
	_, _, ids := seed(t, s) // Clean: old "No Spice"; Steamy: old "Open Door"
	lv := func(n int) *int { return &n }
	sweet, _ := s.UpsertBook("Sweet One", "A", "", "")
	_ = s.SaveAnalysis(sweet, store.Analysis{SpiceLevel: lv(1), Classification: "Open Door"})
	b, _ := s.BookByID(sweet, nil)
	if b.SpiceLevel == nil || *b.SpiceLevel != 1 || *b.Classification != "No Spice" {
		t.Fatalf("peppers should set the label: %+v", b)
	}
	steamy, _ := s.UpsertBook("Closed One", "B", "", "")
	_ = s.SaveAnalysis(steamy, store.Analysis{SpiceLevel: lv(3)})

	kid, _ := s.CreateUserAge("mid", "x", store.RoleRestricted, 2) // middle grade: up to 1 pepper
	if kid.MaxSpice != 1 {
		t.Fatalf("preset max peppers: %d", kid.MaxSpice)
	}
	got := func(u *store.User, f store.BookFilter) map[string]bool {
		books, _, _ := s.ListBooks(f, u)
		return titles(books)
	}
	if v := got(kid, store.BookFilter{}); !v["Sweet One"] || v["Closed One"] || v["Clean"] || v["Steamy"] {
		t.Fatalf("kid up to 1 pepper sees %v (old 'No Spice' counts as up to 2)", v)
	}
	if v := got(nil, store.BookFilter{Spice: "3"}); len(v) != 1 || !v["Closed One"] {
		t.Fatalf("exact pepper filter: %v", v)
	}
	if v := got(nil, store.BookFilter{Spice: "old"}); !v["Clean"] || !v["Steamy"] || v["Sweet One"] {
		t.Fatalf("old-rating filter: %v", v)
	}
	ids2, _ := s.RerateCandidates()
	if len(ids2) != 3 { // Clean, Steamy, Spooky
		t.Fatalf("rerate candidates: %v", ids2)
	}
	_ = ids
}

// A request stays in the history as "deleted" even after the book itself is
// gone (the Calibre re-read removes it before the decision is saved).
func TestDeleteRequestSurvivesBookDeletion(t *testing.T) {
	s := newStore(t)
	_, _, ids := seed(t, s)
	wife, _ := s.CreateUser("wife", "x", store.RoleEditor)
	b, _ := s.BookByID(ids["Steamy"], nil)
	if err := s.RequestDelete(b, wife, "explicit"); err != nil {
		t.Fatal(err)
	}
	reqIDs, _ := s.PendingRequestIDs([]int64{b.ID})
	if _, err := s.DB.Exec(`DELETE FROM books WHERE id = ?`, b.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DecideRequests(reqIDs, "deleted", "admin"); err != nil {
		t.Fatal(err)
	}
	recent, _ := s.RecentDeleteDecisions(5)
	if len(recent) != 1 || recent[0].Title != "Steamy" || recent[0].Status != "deleted" || recent[0].BookID != nil {
		t.Fatalf("history: %+v", recent)
	}
}
