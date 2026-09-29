package store_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Two records of one book become one: the better-rated one stays, the
// other's copies, Up Next places and notes move over, and its title comes
// back to the kept book from then on.
func TestMergeBooks(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	kindle, _ := st.EnsureCatalog("Kindle", "drive")
	hash := "x"
	u, _ := st.CreateUser("parent", hash, store.RoleAdmin)

	orig, _ := st.UpsertBook("A Game of Thrones", "George R. R. Martin", "", "")
	_ = st.AddCopy(cal, orig, "/c/got.epub", "epub", "1")
	two := 2
	_ = st.SaveAnalysis(orig, store.Analysis{SpiceLevel: &two, Model: "manual: parent"})

	alt, _ := st.UpsertBook("Game of Thrones", "George R.R. Martin", "", "")
	_ = st.AddCopy(kindle, alt, "/k/got.azw3", "azw3", "B00")
	_ = st.Enqueue(u.ID, alt)
	_, _ = st.AddNote(alt, u.ID, "Read this together", "everyone")

	kept, err := st.MergeBooks(alt, orig)
	if err != nil || kept != orig {
		t.Fatalf("kept %d (%v), want the parent-rated %d", kept, err, orig)
	}
	if _, err := st.BookByID(alt, nil); err == nil {
		t.Fatal("the other record is still there")
	}
	var copies, queued, notes int
	_ = st.DB.Get(&copies, `SELECT COUNT(*) FROM catalog_books WHERE book_id = ?`, orig)
	_ = st.DB.Get(&queued, `SELECT COUNT(*) FROM queue_items WHERE book_id = ?`, orig)
	_ = st.DB.Get(&notes, `SELECT COUNT(*) FROM book_notes WHERE book_id = ?`, orig)
	if copies != 2 || queued != 1 || notes != 1 {
		t.Fatalf("moved: copies %d, queued %d, notes %d", copies, queued, notes)
	}
	// The other title now finds the kept book instead of making a new one.
	if again, _ := st.UpsertBook("Game of Thrones", "George R.R. Martin", "", ""); again != orig {
		t.Fatalf("alias: got %d, want %d", again, orig)
	}
	if _, err := st.MergeBooks(orig, orig); err == nil {
		t.Fatal("a book can't be merged with itself")
	}
}
