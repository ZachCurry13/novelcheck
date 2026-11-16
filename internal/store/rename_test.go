package store_test

import (
	"errors"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestRenameBook(t *testing.T) {
	s := newStore(t)
	looked, _ := s.EnsureCatalog(store.LookedUpCatalog, "custom")
	id, _ := s.UpsertBook("Abby jiminez a married little Christmas", "Abby Jimenez", "9780349448992", "")
	_ = s.AddCopy(looked, id, "lookup:x", "list", "")

	got, err := s.RenameBook(id, "A Married Little Christmas", "Abby Jimenez")
	if err != nil || got != id {
		t.Fatalf("rename: %d %v", got, err)
	}
	b, _ := s.BookByID(id, nil)
	if b.Title != "A Married Little Christmas" || b.Author != "Abby Jimenez" || b.ISBN != "9780349448992" {
		t.Fatalf("after: %+v", b)
	}
	// Checked again by the old name: the same book, not a new one.
	if again, _ := s.UpsertBook("Abby jiminez a married little Christmas", "Abby Jimenez", "", ""); again != id {
		t.Fatalf("the old name made a new book: %d", again)
	}

	// Renamed to a book NovelCheck already has: one book.
	other, _ := s.UpsertBook("Part of Your World", "Abby Jimenez", "", "")
	typo, _ := s.UpsertBook("Prt of Yuor World", "Abby Jimenez", "", "")
	_ = s.AddCopy(looked, typo, "lookup:y", "list", "")
	kept, err := s.RenameBook(typo, "Part of Your World", "Abby Jimenez")
	if err != nil || (kept != other && kept != typo) {
		t.Fatalf("join: %d %v", kept, err)
	}
	var n int
	_ = s.DB.Get(&n, `SELECT COUNT(*) FROM books WHERE title LIKE '%World%'`)
	if n != 1 {
		t.Fatalf("%d books named World", n)
	}
	if b, _ := s.BookByID(kept, nil); b.Title != "Part of Your World" {
		t.Fatalf("joined title: %+v", b)
	}

	// A Calibre book is changed in Calibre.
	cal, _ := s.EnsureCatalog(store.CalibreCatalogName, "calibre")
	cb, _ := s.UpsertBook("Beach Read", "Emily Henry", "", "")
	_ = s.AddCopy(cal, cb, "/c/beach.epub", "epub", "3")
	if _, err := s.RenameBook(cb, "Beach Reads", "Emily Henry"); !errors.Is(err, store.ErrFromCalibre) {
		t.Fatalf("calibre: %v", err)
	}
	if _, err := s.RenameBook(id, "  ", "x"); err == nil {
		t.Fatal("an empty title was saved")
	}
}
