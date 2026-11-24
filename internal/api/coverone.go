package api

import (
	"context"
	"path/filepath"

	"github.com/zachcurry13/novelcheck/internal/covers"
	"github.com/zachcurry13/novelcheck/internal/hardcover"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// findOneCover looks for one book's cover, exact places first: Open Library by ISBN, the book's own file and
// Amazon by ASIN (coverextra.go), then Hardcover, then a search by title and author (under the true title first
// when the book's looks wrong). An ISBN the search turns up is kept on the book. refused says a place refused to
// answer (Google's daily limit, the network), so "nothing found" is not the last word.
func (s *Server) findOneCover(ctx context.Context, cv covers.Cache, hc *hardcover.Client, b store.BriefBook, backup map[string]string) (ok, refused bool) {
	_, _, ok = s.Covers.OpenLibrary(ctx, firstNonEmpty(b.ISBN, isbn10(b.ASIN))) // the ISBN's cover, when it has one
	if !ok && b.ISBN != "" {                                                    // the same ISBN on Amazon (its ISBN-10)
		ok = s.Covers.ByISBN(ctx, b.ISBN, filepath.Join(s.Covers.Dir, s.coverName(b.ID)+".jpg"), filepath.Join(s.Covers.Dir, s.coverName(b.ID)+".jpg.none"))
	}
	if !ok { // the cover inside the book's own file, or Amazon's by ASIN: exact, so before any search by title
		ok = s.extraCover(ctx, cv, b, backup)
	}
	sug := s.Store.TitleSuggestion(b.ID)
	if !ok && sug == "" { // a wrong title is the usual reason for no cover: find the true one now
		if sug = s.closestTitle(ctx, b.Title, b.Author); sug != "" {
			s.Store.SuggestTitle(b.ID, sug)
			if len(s.Store.CalibreIDs(b.ID)) > 0 {
				s.saveTitleInCalibre(ctx, b.ID, sug)
			} else if _, err := s.Store.RenameBook(b.ID, sug, b.Author); err == nil {
				s.Store.MarkName(b.ID, 1)
			}
		}
	}
	if !ok { // Hardcover has many books the other two don't (self-published, Kindle Unlimited)
		if u := s.hardcoverCover(ctx, hc, b, firstNonEmpty(sug, b.Title)); u != "" {
			ok = s.Covers.Choose(ctx, s.coverName(b.ID), u)
		}
	}
	search := func(title string) bool {
		r := cv.SearchBook(ctx, s.coverName(b.ID), title, b.Author)
		if b.ISBN == "" && s.Store.SetMissingISBN(b.ID, r.ISBN) { // the matching book carried an ISBN: keep it
			b.ISBN = r.ISBN
		}
		refused = refused || r.Refused
		return r.OK
	}
	if !ok && sug != "" { // search under the true title first
		ok = search(sug)
		if !ok {
			s.Covers.Forget(s.coverName(b.ID)) // the miss is for the true title; the old one gets its own try
		}
	}
	if !ok {
		ok = search(b.Title)
	}
	return ok, refused
}
