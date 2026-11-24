package store_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// An entry joined to another book by name can be split out again, and the next read of Calibre keeps it apart.
func TestSplitCalibreEntryStaysSplit(t *testing.T) {
	st := newStore(t)
	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	six := store.CalibreEntry{ExtID: "1", Title: "The Lost Files: Six's Legacy", Authors: "Pittacus Lore"}
	fugitive := store.CalibreEntry{ExtID: "2", Title: "The Fugitive", Authors: "Pittacus Lore"}
	file := func(e store.CalibreEntry) int64 { // what a sync does with an entry: its book, and a link to its file
		id, _ := st.UpsertCalibreBook(cat, e)
		_ = st.AddCopy(cat, id, "/c/"+e.ExtID+".epub", "epub", e.ExtID)
		return id
	}
	a, b := file(six), file(fugitive)
	if a == b {
		t.Fatal("two titles, two books")
	}
	kept, err := st.MergeBooks(a, b) // they were joined (the old ISBN rule, or by hand)
	if err != nil {
		t.Fatal(err)
	}
	if again := file(fugitive); again != kept {
		t.Fatalf("the join should hold the Fugitive on the same book: %d vs %d", again, kept)
	}
	if err := st.SplitCalibreEntry("2", fugitive.Title, fugitive.Authors); err != nil {
		t.Fatal(err)
	}
	apart := file(fugitive) // the next read of Calibre
	same := file(six)
	if apart == same {
		t.Fatal("the Fugitive should be a book of its own")
	}
	if err := st.SplitCalibreEntry("1", six.Title, six.Authors); err == nil {
		t.Log("(the other entry holds the book's own name or was split too)")
	}
}
