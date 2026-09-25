package store_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestCalibreTitlesAndRenames(t *testing.T) {
	st := newStore(t)
	cat, _ := st.EnsureCatalog("Calibre Main", "calibre")
	one := 1
	entry := store.CalibreEntry{ExtID: "7", Title: "01 - Guards! Guards!", Authors: "Terry Pratchett"}
	id, err := st.UpsertCalibreBook(cat, entry)
	if err != nil {
		t.Fatal(err)
	}
	_ = st.AddCalibreCopy(cat, id, "/calibre/Terry Pratchett/01 - Guards! Guards! (7)/x.epub", "epub", "7", "2026-01-01 10:00:00")
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &one, Model: "gpt"})
	b, _ := st.BookByID(id, nil)
	if b.Title != "Guards! Guards!" || b.SeriesIndex != 1 || b.TitleFix != "01 - Guards! Guards!" {
		t.Fatalf("tidy title, number kept, Calibre's title remembered: %+v", b)
	}
	if fixes, _ := st.TitleFixes(); len(fixes) != 1 || fixes[0].CalibreTitle != "01 - Guards! Guards!" || st.CountTitleFixes() != 1 {
		t.Fatalf("offered for fixing: %+v", fixes)
	}
	if ids := st.CalibreIDs(id); len(ids) != 1 || ids[0] != 7 {
		t.Fatalf("calibre ids: %v", ids)
	}

	// Calibre's own series wins; once the title is tidy there, nothing's left to fix.
	entry.Title, entry.Series, entry.SeriesIndex = "Guards! Guards!", "Discworld", 8
	if again, _ := st.UpsertCalibreBook(cat, entry); again != id {
		t.Fatal("same book")
	}
	b, _ = st.BookByID(id, nil)
	if b.Series != "Discworld" || b.SeriesIndex != 8 || b.TitleFix != "" || st.CountTitleFixes() != 0 {
		t.Fatalf("series from Calibre: %+v", b)
	}

	// Renamed in Calibre: the same book follows (rating kept), and the moved
	// file replaces the old link.
	entry.Title = "Guards Guards (Anniversary Edition)"
	renamed, _ := st.UpsertCalibreBook(cat, entry)
	b, _ = st.BookByID(renamed, nil)
	if renamed != id || b.Title != entry.Title || b.Status != "analyzed" {
		t.Fatalf("rename followed: %d vs %d, %+v", renamed, id, b)
	}
	newPath := "/calibre/Terry Pratchett/Guards Guards (7)/x.epub"
	_ = st.AddCalibreCopy(cat, id, newPath, "epub", "7", "2026-01-01 10:00:00")
	if err := st.PruneStaleCopies(cat, map[store.CopyKey]bool{{Book: id, Path: newPath}: true}); err != nil {
		t.Fatal(err)
	}
	if copies, _ := st.BookCopies(id); len(copies) != 1 || copies[0].Path != newPath {
		t.Fatalf("stale link dropped: %+v", copies)
	}

	// A new name that belongs to another book joins that book instead.
	other, _ := st.UpsertBook("Mort", "Terry Pratchett", "", "")
	entry.Title = "Mort"
	if joined, _ := st.UpsertCalibreBook(cat, entry); joined != other {
		t.Fatal("matches the existing book")
	}
}

func TestTidyTitlesAndSavedToCalibre(t *testing.T) {
	st := newStore(t)
	cat, _ := st.EnsureCatalog("Calibre Main", "calibre")
	one := 1
	// A book stored before tidying (as older versions did).
	res, _ := st.DB.Exec(`INSERT INTO books (norm_key, title, author) VALUES (?, ?, ?)`, "02thetwotowers|tolkien", "02 - The Two Towers", "J.R.R. Tolkien")
	id, _ := res.LastInsertId()
	_ = st.AddCalibreCopy(cat, id, "/calibre/a.epub", "epub", "12", "2026-01-01 10:00:00")
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &one, Model: "gpt"})
	if n, err := st.TidyTitles(); err != nil || n != 1 {
		t.Fatalf("tidied %d, %v", n, err)
	}
	if n, _ := st.TidyTitles(); n != 0 {
		t.Fatal("runs once")
	}
	b, _ := st.BookByID(id, nil)
	if b.Title != "The Two Towers" || b.SeriesIndex != 2 || b.TitleFix != "02 - The Two Towers" || st.MatchBook("The Two Towers", "Tolkien") != id {
		t.Fatalf("tidied: %+v", b)
	}

	// After the admin saves it in Calibre, Calibre stamps a new change time;
	// our own edit isn't a reason to re-rate.
	if err := st.SavedToCalibre(id, "The Two Towers", "The Lord of the Rings", 2); err != nil {
		t.Fatal(err)
	}
	b, _ = st.BookByID(id, nil)
	if b.TitleFix != "" || b.Series != "The Lord of the Rings" || b.SeriesIndex != 2 {
		t.Fatalf("saved: %+v", b)
	}
	_ = st.AddCalibreCopy(cat, id, "/calibre/a.epub", "epub", "12", "2026-05-01 10:00:00")
	if !st.ChangedInCalibre(id) {
		t.Fatal("looks changed before accepting")
	}
	_ = st.AcceptModified([]int64{id})
	if st.ChangedInCalibre(id) || st.ChangedSinceRated() != 0 {
		t.Fatal("our own edit accepted")
	}
}
