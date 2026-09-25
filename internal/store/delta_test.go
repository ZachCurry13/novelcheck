package store_test

import (
	"slices"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestDeltaScanning(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	cat, _ := st.EnsureCatalog("Calibre Main", "calibre")
	one := 1
	id, _ := st.UpsertBook("Edited Later", "A", "", "")
	_ = st.AddCalibreCopy(cat, id, "/calibre/a.epub", "epub", "1", "2026-01-01 10:00:00")
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &one, Model: "gpt"})

	candidates := func() []int64 { ids, _ := st.RerateCandidates(); return ids }
	if st.ChangedSinceRated() != 0 || slices.Contains(candidates(), id) {
		t.Fatal("an unchanged book is never rated twice")
	}
	// A re-sync with the same time changes nothing; a later time means Calibre changed it.
	_ = st.AddCalibreCopy(cat, id, "/calibre/a.epub", "epub", "1", "2026-01-01 10:00:00")
	if st.ChangedSinceRated() != 0 {
		t.Fatal("same change time")
	}
	_ = st.AddCalibreCopy(cat, id, "/calibre/a.epub", "epub", "1", "2026-02-15 08:30:00")
	if st.ChangedSinceRated() != 1 || !slices.Contains(candidates(), id) {
		t.Fatal("a book changed in Calibre is offered for re-rating")
	}
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &one, Model: "gpt"})
	if st.ChangedSinceRated() != 0 {
		t.Fatal("re-rating catches up")
	}

	// Ratings from before delta scanning get a baseline instead of all looking changed.
	_, _ = st.DB.Exec(`UPDATE books SET rated_modified = '' WHERE id = ?`, id)
	_ = st.AddCalibreCopy(cat, id, "/calibre/a.epub", "epub", "1", "2026-03-01 00:00:00")
	if err := st.BaselineModified(); err != nil {
		t.Fatal(err)
	}
	if st.ChangedSinceRated() != 0 {
		t.Fatal("baseline")
	}
	// Hand ratings are the parent's call: never offered.
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &one, Model: "manual: mom"})
	_ = st.AddCalibreCopy(cat, id, "/calibre/a.epub", "epub", "1", "2026-04-01 00:00:00")
	if st.ChangedSinceRated() != 0 {
		t.Fatal("hand ratings stay")
	}
}
