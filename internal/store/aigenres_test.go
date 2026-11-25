package store_test

import "testing"

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
}
