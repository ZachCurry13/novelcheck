package api_test

import (
	"fmt"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestBoxSetSplit(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	box, _ := st.UpsertBook("The Hunger Games Trilogy Box Set", "Suzanne Collins", "", "")
	_ = st.AddCopy(cal, box, "/c/hg-box.epub", "epub", "10")
	_, _ = st.DB.Exec(`UPDATE books SET series = 'The Hunger Games' WHERE id = ?`, box)
	first, _ := st.UpsertBook("The Hunger Games", "Suzanne Collins", "", "")
	_ = st.AddCopy(cal, first, "/c/hg1.epub", "epub", "11")
	plain, _ := st.UpsertBook("Charlotte's Web", "E. B. White", "", "")
	_ = st.AddCopy(cal, plain, "/c/cw.epub", "epub", "12")

	res, list := admin.doList("GET", "/api/box-sets")
	if res.StatusCode != 200 || len(list) != 1 || int64(list[0]["book_id"].(float64)) != box {
		t.Fatalf("found: %d %v", res.StatusCode, list)
	}
	books := []store.BoxEntry{{Title: "The Hunger Games", Number: 1}, {Title: "Catching Fire", Number: 2}, {Title: "Mockingjay", Number: 3}}
	res, out := admin.do("POST", fmt.Sprintf("/api/box-sets/%d/split", box), map[string]any{"books": books}, true)
	if res.StatusCode != 200 || out["rating"].(float64) != 3 {
		t.Fatalf("split: %d %v", res.StatusCode, out)
	}
	fire, _ := st.UpsertBook("Catching Fire", "Suzanne Collins", "", "")
	if !st.Owned(fire) {
		t.Fatal("a split book counts as owned through the box set")
	}
	if b, _ := st.BookByID(fire, nil); b.Series != "The Hunger Games" || b.SeriesIndex != 2 {
		t.Fatalf("series: %q %v", b.Series, b.SeriesIndex)
	}
	copies, _ := st.BookCopies(fire)
	own, _ := st.OwnCopies(fire)
	if len(copies) != 1 || copies[0].FromBox == "" || copies[0].Path != "/c/hg-box.epub" || len(own) != 0 {
		t.Fatalf("copies %+v own %+v", copies, own)
	}
	// The box set leaves the Library; its books are there.
	if _, out = admin.do("GET", "/api/books?q=Trilogy", nil, false); out["total"].(float64) != 0 {
		t.Fatalf("box set still listed: %v", out["total"])
	}
	if _, out = admin.do("GET", "/api/books?q=Mockingjay", nil, false); out["total"].(float64) != 1 {
		t.Fatalf("split book not listed: %v", out["total"])
	}
	// Undo: the books the split made go; one the family had stays.
	if res, _ := admin.do("POST", fmt.Sprintf("/api/box-sets/%d/undo", box), nil, true); res.StatusCode != 200 {
		t.Fatalf("undo: %d", res.StatusCode)
	}
	if _, err := st.BookByID(fire, nil); err == nil {
		t.Fatal("a book made by the split is still there")
	}
	if _, err := st.BookByID(first, nil); err != nil {
		t.Fatal("undo removed a book the family had")
	}
	if res, _ := admin.do("POST", fmt.Sprintf("/api/box-sets/%d/not", box), nil, true); res.StatusCode != 200 {
		t.Fatalf("not: %d", res.StatusCode)
	}
	// Asking the AI about a set that is no longer waiting answers at once, without calling the AI.
	if res, _ := admin.do("POST", fmt.Sprintf("/api/box-sets/%d/ask", box), nil, true); res.StatusCode != 409 {
		t.Fatalf("ask after a decision: %d, want 409", res.StatusCode)
	}
	if _, list = admin.doList("GET", "/api/box-sets"); len(list) != 0 {
		t.Fatalf("dismissed box set came back: %v", list)
	}
}

// A box set's inside title that is only worded a little differently from a book the family has
// ("Sea of Monsters" for "The Sea of Monsters") joins that book instead of making a duplicate, and the
// review window can ask which book a title would be.
func TestBoxSplitJoinsExistingBooks(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	box, _ := st.UpsertBook("Percy Jackson and the Olympians 3 Books Collection Set", "Rick Riordan", "", "")
	_ = st.AddCopy(cal, box, "/c/pj-box.epub", "epub", "20")
	sea, _ := st.UpsertBook("The Sea of Monsters", "Rick Riordan", "", "")
	_ = st.AddCopy(cal, sea, "/c/sea.epub", "epub", "21")

	admin.doList("GET", "/api/box-sets") // finds the box set, as opening the window does
	_, m := admin.do("GET", fmt.Sprintf("/api/box-sets/%d/match?title=%s", box, "Sea%20of%20Monsters"), nil, false)
	if int64(m["match_id"].(float64)) != sea || m["match_title"] != "The Sea of Monsters" || m["match_owned"] != true {
		t.Fatalf("the review window learns it is the book already owned: %v", m)
	}
	if _, m = admin.do("GET", fmt.Sprintf("/api/box-sets/%d/match?title=%s", box, "The%20Chalice%20of%20the%20Gods"), nil, false); m["match_id"] != nil {
		t.Fatalf("a title nothing matches is a new book: %v", m)
	}
	books := []store.BoxEntry{{Title: "Sea of Monsters", Number: 2}, {Title: "The Chalice of the Gods", Number: 6}}
	if res, _ := admin.do("POST", fmt.Sprintf("/api/box-sets/%d/split", box), map[string]any{"books": books}, true); res.StatusCode != 200 {
		t.Fatalf("split: %d", res.StatusCode)
	}
	var n int
	st.DB.Get(&n, `SELECT COUNT(*) FROM books WHERE title LIKE '%Sea of Monsters%'`)
	if n != 1 {
		t.Fatalf("no duplicate of the book the family already had: %d", n)
	}
	// Undoing keeps the one that was there, drops only the one the split made.
	admin.do("POST", fmt.Sprintf("/api/box-sets/%d/undo", box), nil, true)
	if _, err := st.BookByID(sea, nil); err != nil {
		t.Fatal("undo removed a book the family had")
	}
	st.DB.Get(&n, `SELECT COUNT(*) FROM books WHERE title LIKE '%Chalice%'`)
	if n != 0 {
		t.Fatalf("the book the split made went: %d", n)
	}
}
