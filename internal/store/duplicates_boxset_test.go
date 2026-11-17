package store_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// A split box set never makes its books look like Calibre duplicates: the books of a split use the box set's files
// through a link (box_members), they don't get its Calibre entries. So the real single-book entry and the omnibus
// entry are different books to "Duplicates in Calibre", whether the book was already in the library (the original,
// created = 0) or was made by the split (created = 1).
func TestSplitBoxSetIsNotACalibreDuplicate(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	add := func(ext, title, author string) int64 {
		id, err := st.UpsertCalibreBook(cal, store.CalibreEntry{ExtID: ext, Title: title, Authors: author})
		if err != nil {
			t.Fatal(err)
		}
		_ = st.AddCalibreCopy(cal, id, "/c/"+ext+".epub", "epub", ext, "2026-01-01 10:00:00")
		return id
	}
	box := add("200", "The Dark Tower: Books 1-2 Omnibus", "Stephen King")
	original := add("100", "The Gunslinger", "Stephen King") // already in the library, with its own Calibre entry
	if n, err := st.FindBoxSets(); err != nil || n == 0 {
		t.Fatalf("the omnibus is found as a box set: %d %v", n, err)
	}
	if _, err := st.SplitBox(box, []store.BoxEntry{{Title: "The Gunslinger", Number: 1}, {Title: "The Drawing of the Three", Number: 2}}, "test"); err != nil {
		t.Fatal(err)
	}
	var created map[int64]bool = map[int64]bool{}
	rows, _ := d.Query(`SELECT book_id, created FROM box_members WHERE box_id = ?`, box)
	for rows.Next() {
		var id int64
		var c bool
		_ = rows.Scan(&id, &c)
		created[id] = c
	}
	rows.Close()
	if c, ok := created[original]; !ok || c {
		t.Fatalf("the book that was already there is a member that was not created by the split: %v", created)
	}
	if len(created) != 2 {
		t.Fatalf("two members, one made by the split: %v", created)
	}
	if groups, err := st.CalibreDuplicates(); err != nil || len(groups) != 0 {
		t.Fatalf("a split box set and its books are not Calibre duplicates: %+v %v", groups, err)
	}
	// A real second Calibre entry of the original is a duplicate, and only of the original: the box set's entry
	// is never in that group.
	add("101", "The Gunslinger", "Stephen King")
	groups, err := st.CalibreDuplicates()
	if err != nil || len(groups) != 1 || groups[0].BookID != original || len(groups[0].Entries) != 2 {
		t.Fatalf("the original's two entries: %+v %v", groups, err)
	}
	for _, e := range groups[0].Entries {
		if e.CalibreID == "200" {
			t.Fatal("the box set's Calibre entry must never be in the original's duplicate group")
		}
	}
}
