package store_test

import (
	"strings"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// A rating is kept when its book is deleted (it left every library, list and event) and comes back when the same
// book does, with its content items and a parent's OK: nothing is rated twice. A book that was never rated is
// not given a rating it never had.
func TestRatingComesBackWithTheBook(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	id, _ := st.UpsertBook("Quick Free Read", "Pat Free", "", "A blurb")
	two := 2
	if err := st.SaveAnalysis(id, store.Analysis{SpiceLevel: &two, Classification: "Closed Door", Nudity: true, Confidence: "low", Model: "llama3.2",
		SummaryVerdict: "A sweet read.", Premise: "A girl finds a key.", Content: []string{"fights"}, ContentSource: "ai"}); err != nil {
		t.Fatal(err)
	}
	_, _ = d.Exec(`UPDATE books SET approved = 1, approved_by = 'mum' WHERE id = ?`, id)
	pending, _ := st.UpsertBook("Never Rated", "Pat Free", "", "")

	// Both books leave everything and are deleted.
	if _, err := d.Exec(`DELETE FROM books WHERE id IN (?, ?)`, id, pending); err != nil {
		t.Fatal(err)
	}
	again, _ := st.UpsertBook("Quick Free Read", "Pat Free", "", "A blurb")
	b, err := st.BookByID(again, nil)
	if err != nil {
		t.Fatal(err)
	}
	if b.Status != "analyzed" || b.SpiceLevel == nil || *b.SpiceLevel != 2 || b.Confidence != "low" || !b.Nudity || !b.Approved ||
		b.ApprovedBy != "mum" || b.AnalysisModel != "llama3.2" || b.Premise != "A girl finds a key." {
		t.Fatalf("the rating comes back with the book: %+v", b)
	}
	var n int
	_ = d.Get(&n, `SELECT COUNT(*) FROM book_content WHERE book_id = ? AND key = 'fights' AND source = 'ai'`, again)
	if n != 1 {
		t.Fatal("its content items come back too")
	}
	if again == id {
		t.Fatal("a new row, not the old one")
	}
	if prev := st.PreviousBookIDs(again); len(prev) != 1 || prev[0] != id {
		t.Fatalf("the old id is known (its cover is stored under it): %v", prev)
	}
	never, _ := st.UpsertBook("Never Rated", "Pat Free", "", "")
	if b, _ := st.BookByID(never, nil); b == nil || b.Status != "pending" {
		t.Fatalf("a book that was never rated stays waiting: %+v", b)
	}
	// Deleted again, still remembered (the memory follows the latest rating).
	_, _ = d.Exec(`DELETE FROM books WHERE id = ?`, again)
	third, _ := st.UpsertBook("Quick Free Read", "Pat Free", "", "A blurb")
	if b, _ := st.BookByID(third, nil); b == nil || b.Status != "analyzed" {
		t.Fatalf("remembered more than once: %+v", b)
	}
}

// Remembering can be turned off (nothing is kept, nothing comes back), and remembered ratings can be forgotten, all
// of them or only the old ones.
func TestRememberingCanBeTurnedOffAndForgotten(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	one := 1
	rate := func(title string) int64 {
		id, _ := st.UpsertBook(title, "Pat Free", "", "")
		_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &one, Confidence: "high", Model: "llama3.2"})
		return id
	}
	_ = st.SetSetting(store.KeyRememberRatings, "false")
	id := rate("Off Book")
	_, _ = d.Exec(`DELETE FROM books WHERE id = ?`, id)
	if n, _ := st.MemoryStats(); n != 0 {
		t.Fatalf("nothing is kept while it is off: %d", n)
	}
	if b, _ := st.BookByID(func() int64 { x, _ := st.UpsertBook("Off Book", "Pat Free", "", ""); return x }(), nil); b.Status != "pending" {
		t.Fatalf("and nothing comes back: %q", b.Status)
	}
	_ = st.SetSetting(store.KeyRememberRatings, "true")
	for _, title := range []string{"Old One", "Recent One"} {
		id := rate(title)
		_, _ = d.Exec(`DELETE FROM books WHERE id = ?`, id)
	}
	_, _ = d.Exec(`UPDATE rating_memory SET saved_at = ? WHERE norm_key LIKE 'oldone%'`, time.Now().AddDate(0, 0, -400).Unix())
	if n, _ := st.MemoryStats(); n != 2 {
		t.Fatalf("two remembered: %d", n)
	}
	if got := st.ForgetRatings(time.Now().AddDate(0, 0, -365)); got != 1 {
		t.Fatalf("only the old one is forgotten: %d", got)
	}
	if n, _ := st.MemoryStats(); n != 1 {
		t.Fatalf("one left: %d", n)
	}
	if got := st.ForgetRatings(time.Time{}); got != 1 {
		t.Fatalf("then all: %d", got)
	}
}

// A library book is found by its title alone, whatever the author, ignoring punctuation (a Kindle file named
// Some-Title.epub must not add a second copy).
func TestLibraryBookByTitleKey(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	id, _ := st.UpsertBook("Harry Potter and the Chamber of Secrets", "J. K. Rowling", "", "")
	_ = st.AddCopy(cal, id, "/c/hp2.epub", "epub", "1")
	key, _, _ := strings.Cut(store.NormKey("Harry-Potter-and-the-Chamber-of-Secrets", ""), "|")
	if st.LibraryBookByTitleKey(key) != id {
		t.Fatal("found by title alone")
	}
	if st.LibraryBookByTitleKey("nosuchbook") != 0 {
		t.Fatal("an unknown title is not found")
	}
}

// A Calibre entry removed on the Content server is forgotten at once (its book too, when it has no other copy), and
// a "From Hardcover" answer is kept in the database.
func TestDropCalibreEntriesAndKeptHardcoverAnswer(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	a, _ := st.UpsertBook("Two Copies", "Pat", "", "")
	_ = st.AddCopy(cal, a, "/c/a1.epub", "epub", "11")
	_ = st.AddCopy(cal, a, "/c/a2.epub", "epub", "12")
	b, _ := st.UpsertBook("One Copy", "Pat", "", "")
	_ = st.AddCopy(cal, b, "/c/b.epub", "epub", "13")
	st.DropCalibreEntries([]int{11, 13})
	if _, err := st.BookByID(a, nil); err != nil {
		t.Fatal("a book with another copy stays")
	}
	if _, err := st.BookByID(b, nil); err == nil {
		t.Fatal("a book with no copy left goes")
	}
	st.HCMoreSave(77, `{"similar":[]}`)
	if data, at, ok := st.HCMoreGet(77); !ok || data != `{"similar":[]}` || time.Since(at) > time.Minute {
		t.Fatalf("the kept answer comes back: %q %v %v", data, at, ok)
	}
}

// File-name habits are undone before a title is compared: a Calibre number, a library-style ", The", hyphens.
func TestTidyBookTitle(t *testing.T) {
	for in, want := range map[string]string{
		"Fall of Five, The":            "The Fall of Five",
		"Pittacus Lore (4006)":         "Pittacus Lore",
		"The-Fall-of-Five":             "The Fall of Five",
		"A Wrinkle in Time":            "A Wrinkle in Time",
		"Twenty-One Days":              "Twenty-One Days",
		"Hunger Games, The (12)":       "The Hunger Games",
		"Lion_the_Witch_Wardrobe, The": "The Lion the Witch Wardrobe",
	} {
		if got := store.TidyBookTitle(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}
