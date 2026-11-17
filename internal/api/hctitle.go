package api

import (
	"context"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/covers"
	"github.com/zachcurry13/novelcheck/internal/hardcover"
	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/titles"
)

// Hardcover as the source of true titles (the owner's rule: Hardcover is the truth). The name finder
// (namefinder.go) only asks Open Library, which doesn't carry self-published and Kindle Unlimited books, and
// accepts only typo-sized fixes; so "Descent into the Void (An action packed science fiction adventure)" was
// never touched. The Hardcover passes (ratings, covers, the Audiobookshelf cleanup) already find these books;
// when the book Hardcover found is this one under a tidier title, that title is used.

// hcTrueTitle is Hardcover's title for the book when it is the same book written differently: the title Hardcover
// has equals one of this title's tidied forms (without the author's name in front, a series in brackets, a
// numbered series prefix, or a subtitle) by the same author. "" when it is already right or isn't clearly the
// same book (an ISBN that leads elsewhere never renames anything).
func hcTrueTitle(title, author string, info hardcover.BookInfo) string {
	t := strings.TrimSpace(info.Title)
	if t == "" || len(t) > 200 || t == strings.TrimSpace(title) {
		return ""
	}
	for _, v := range covers.Titles(titles.WithoutAuthor(title, author)) {
		for _, a := range info.Authors {
			if store.NormKey(v, author) == store.NormKey(t, a) {
				return t
			}
		}
	}
	return ""
}

// applyTrueTitle gives the book its true title: a Calibre book gets a suggestion that is saved in Calibre by
// itself (unless that is turned off), any other book is renamed. A name a parent typed is never changed.
func (s *Server) applyTrueTitle(ctx context.Context, id int64, author, title string) bool {
	if s.Store.NameTyped(id) {
		return false
	}
	if len(s.Store.CalibreIDs(id)) > 0 {
		s.Store.SuggestTitle(id, title)
		s.saveTitleInCalibre(ctx, id, title)
		return true
	}
	kept, err := s.Store.RenameBook(id, title, author)
	if err == nil {
		s.Store.MarkName(kept, 1)
	}
	return err == nil
}

// hcInfoByTitle finds a book on Hardcover by title and author, then by the title's tidied forms (a messy title
// like "Loving the Lonely Viscount: A Chasing Love Novella (Chasing Love Series)" is found as "Loving the Lonely
// Viscount"). Each needs an exact title and author match on Hardcover's side.
func hcInfoByTitle(ctx context.Context, hc *hardcover.Client, title, author string) (hardcover.BookInfo, bool, error) {
	tried := map[string]bool{}
	for _, t := range append([]string{title}, covers.Titles(titles.WithoutAuthor(title, author))...) {
		if k := strings.ToLower(strings.TrimSpace(t)); k == "" || tried[k] {
			continue
		} else {
			tried[k] = true
		}
		if info, ok, err := hc.InfoByTitle(ctx, t, author); err != nil || ok {
			return info, ok, err
		}
	}
	return hardcover.BookInfo{}, false, nil
}
