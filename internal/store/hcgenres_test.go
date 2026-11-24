package store_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestHardcoverGenresFirst(t *testing.T) {
	st := newStore(t)
	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	bare, _ := st.UpsertCalibreBook(cat, store.CalibreEntry{ExtID: "1", Title: "Mort"})
	tagged, _ := st.UpsertCalibreBook(cat, store.CalibreEntry{ExtID: "2", Title: "Dune", Tags: []string{"Science Fiction"}})
	st.SetHCGenres(bare, []string{"Fantasy"})
	st.SetHCGenres(tagged, []string{"Fantasy"})
	if b, _ := st.BookByID(bare, nil); b.GenreSource != "hardcover" || b.Genres != ",fantasy," {
		t.Fatalf("untagged book: %q %q", b.GenreSource, b.Genres)
	}
	if b, _ := st.BookByID(tagged, nil); b.GenreSource != "calibre" {
		t.Fatalf("Calibre's own genres were replaced: %q", b.GenreSource)
	}
	// a later Calibre sync without tags keeps Hardcover's genres
	_, _ = st.UpsertCalibreBook(cat, store.CalibreEntry{ExtID: "1", Title: "Mort"})
	if b, _ := st.BookByID(bare, nil); b.GenreSource != "hardcover" {
		t.Fatalf("sync dropped Hardcover genres: %q", b.GenreSource)
	}
}
