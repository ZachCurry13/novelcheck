package store_test

import (
	"testing"
	"time"
)

// A book whose genres the AI made is asked about on Hardcover again (2.1.10), so Hardcover's can replace them.
func TestRecheckAIGenres(t *testing.T) {
	st := newStore(t)
	cal, _ := st.EnsureCatalog("Calibre Main", "calibre")
	id, _ := st.UpsertBook("Genre Book", "Someone", "", "")
	_ = st.AddCopy(cal, id, "/c/g/genre.epub", "epub", "1")
	if _, err := st.DB.Exec(`UPDATE books SET genre_source = 'ai', genres = 'romance', hc_rated_at = 5 WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if g := st.GenreSources(); g["ai"] != 1 {
		t.Fatalf("sources: %v", g)
	}
	st.RecheckAIGenres()
	var at int64
	_ = st.DB.Get(&at, `SELECT hc_rated_at FROM books WHERE id = ?`, id)
	if at != 0 {
		t.Fatalf("asked again: %d", at)
	}
}

// The same cleanup note again (only its numbers differ) updates the newest line instead of adding one.
func TestCleanupLogFoldsRepeats(t *testing.T) {
	st := newStore(t)
	st.AddCleanupLog("Cover finder", "no cover found for 89 of 89 books. Some: A | B")
	st.AddCleanupLog("Cover finder", "no cover found for 88 of 88 books. Some: A | B")
	st.AddCleanupLog("Cover finder", "no cover found for 5 of 5 books. Some: C")
	if n := len(st.CleanupLog(10)); n != 2 {
		t.Fatalf("entries: %d", n)
	}
	// other actions keep every line: their numbers are different books
	st.AddCleanupLog("POST /api/admin/title-fixes", `asked ids [1] answer skipped`)
	st.AddCleanupLog("POST /api/admin/title-fixes", `asked ids [2] answer skipped`)
	if n := len(st.CleanupLog(10)); n != 4 {
		t.Fatalf("entries kept: %d", n)
	}
}

// A book is left out of the cover lists only on a second sighting at least a day later (one search can miss a cover when a
// site is down).
func TestNothingFoundNeedsTwoSightings(t *testing.T) {
	st := newStore(t)
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	if got := st.ConfirmNothingFound([]int64{1, 2}, now, 20*time.Hour); len(got) != 0 {
		t.Fatalf("first sighting: %v", got)
	}
	if got := st.ConfirmNothingFound([]int64{1, 2}, now.Add(2*time.Hour), 20*time.Hour); len(got) != 0 {
		t.Fatalf("same day: %v", got)
	}
	if got := st.ConfirmNothingFound([]int64{1}, now.Add(24*time.Hour), 20*time.Hour); len(got) != 1 || got[0] != 1 {
		t.Fatalf("a day later, only the one still missing: %v", got)
	}
}
