package store_test

import (
	"testing"

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
