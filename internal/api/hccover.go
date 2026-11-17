package api

import (
	"context"

	"github.com/zachcurry13/novelcheck/internal/covers"
	"github.com/zachcurry13/novelcheck/internal/hardcover"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// hardcoverCover asks Hardcover for a book's cover, for the cover finder (coverfinder.go): by the book's
// Hardcover id when the ratings pass found it, else by its title and author (the title tidied the way the
// cover search tidies it, and only an exact title and author match counts). Many self-published and Kindle
// Unlimited books Open Library and Google Books don't have are on Hardcover. "" when there's no Hardcover
// connection or no cover.
func (s *Server) hardcoverCover(ctx context.Context, hc *hardcover.Client, b store.BriefBook, title string) string {
	if hc == nil {
		return ""
	}
	if id := s.Store.HCIDOf(b.ID); id > 0 {
		if u, err := hc.ImageByID(ctx, id); err == nil && covers.AllowedImage(u) {
			return u
		}
	}
	info, ok, err := hcInfoByTitle(ctx, hc, title, b.Author)
	if err != nil || !ok {
		return ""
	}
	s.Store.SaveHCID(b.ID, info.ID)
	if t := hcTrueTitle(b.Title, b.Author, info); t != "" {
		s.applyTrueTitle(ctx, b.ID, b.Author, t)
	}
	if covers.AllowedImage(info.Image) {
		return info.Image
	}
	return ""
}
