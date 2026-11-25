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
