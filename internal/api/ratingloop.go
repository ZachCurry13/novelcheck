package api

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/hardcover"
	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/titles"
)

// Hardcover ratings for every book (store/hcratings.go): looked up in the background with a family member's
// Hardcover connection, by ISBN 25 at a time, then by title and author (a messy title by the true one), about
// one request a second. A book Hardcover doesn't know is looked at again after a month. A book with no ISBN gets
// one on the way (Hardcover's, else Open Library's when it is clearly the same book), saved in Calibre too.

// FillRatings looks up to limit books up and says how many got a rating.
func (s *Server) FillRatings(ctx context.Context, limit int) int {
	token := s.Store.RatingsToken()
	if token == "" {
		return 0
	}
	books, err := s.Store.RatingsToCheck(limit)
	if err != nil || len(books) == 0 {
		return 0
	}
	hc := &hardcover.Client{Token: token, URL: hardcoverURL}
	found, renamed := 0, 0
	defer func() {
		if renamed > 0 {
			log.Printf("hardcover: %d books got their true title", renamed)
		}
	}()
	keep := func(b store.RatingToCheck, info hardcover.BookInfo) {
		s.Store.SaveHCRating(b.ID, info.Rating, info.Ratings)
		s.Store.SaveHCID(b.ID, info.ID)
		if t := hcTrueTitle(b.Title, b.Author, info); t != "" && s.applyTrueTitle(ctx, b.ID, b.Author, t) {
			renamed++
		}
		s.Store.SetHCGenres(b.ID, info.Tags)
		s.Store.SaveHCTags(b.ID, strings.Join(info.Warnings, ", "), strings.Join(info.Moods, ", "))
		if info.Rating > 0 {
			found++
		}
		if b.ISBN == "" { // a book with no ISBN gets Hardcover's (or, when Hardcover doesn't know it, Open Library's)
			isbn := firstNonEmpty(info.ISBN13, info.ISBN10)
			if isbn == "" {
				isbn = s.openLibraryISBN(ctx, b.Title, b.Author)
			}
			if s.Store.SetMissingISBN(b.ID, isbn) {
				s.saveISBNInCalibre(ctx, b.ID, isbn)
			}
		}
	}
	var rest []store.RatingToCheck // no ISBN, or Hardcover doesn't know it
	for i := 0; i < len(books); i += 25 {
		if ctx.Err() != nil {
			return found
		}
		chunk := books[i:min(i+25, len(books))]
		var isbns []string
		for _, b := range chunk {
			if d := digitsOnly(b.ISBN); d != "" {
				isbns = append(isbns, d)
			}
		}
		byISBN, err := hc.InfoByISBN(ctx, isbns)
		if err != nil {
			log.Printf("hardcover ratings: %v", err)
			return found
		}
		for _, b := range chunk {
			if info, ok := byISBN[digitsOnly(b.ISBN)]; ok && b.ISBN != "" {
				keep(b, info)
			} else {
				rest = append(rest, b)
			}
		}
		time.Sleep(time.Second)
	}
	for _, b := range rest {
		if ctx.Err() != nil {
			break
		}
		info, ok, err := hcInfoByTitle(ctx, hc, b.Title, b.Author)
		if err == nil && !ok {
			if t := s.closestTitle(ctx, b.Title, b.Author); t != "" { // a typo or dots in the title
				info, ok, err = hc.InfoByTitle(ctx, t, b.Author)
			}
		}
		if err != nil {
			log.Printf("hardcover ratings: %v", err)
			break
		}
		if !ok {
			info = hardcover.BookInfo{} // not on Hardcover: asked again in a month
		}
		keep(b, info)
		time.Sleep(time.Second)
	}
	return found
}

const hcTitlesKey = "hc_titles_pass"
const hcGenresKey = "hc_genres_pass"

// RatingLoop fills ratings 12 minutes after start, then every 6 hours, 800 books a round.
func (s *Server) RatingLoop(ctx context.Context) {
	wait := 12 * time.Minute
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = 6 * time.Hour
		if s.Store.Setting(hcTitlesKey) != "1" { // the pass learned the true titles: look at every book again
			s.Store.RecheckHardcover()
			_ = s.Store.SetSetting(hcTitlesKey, "1")
		}
		if s.Store.Setting(hcGenresKey) != "1" { // genres from Hardcover (2.1.4): the books without one are looked at again
			s.Store.RecheckGenreless()
			_ = s.Store.SetSetting(hcGenresKey, "1")
		}
		if n := s.FillRatings(ctx, 800); n > 0 {
			log.Printf("hardcover ratings: %d books rated", n)
		}
		s.cleanAudiobooksNow(300) // the Audiobookshelf cleanup (absclean.go), with the same Hardcover connection
	}
}

func firstNonEmpty(v ...string) string {
	for _, x := range v {
		if x = strings.TrimSpace(x); x != "" {
			return x
		}
	}
	return ""
}

// openLibraryISBN is a book's ISBN from Open Library, only when Open Library's book is clearly this one (the same
// author, and the same title allowing a typo); "" otherwise.
func (s *Server) openLibraryISBN(ctx context.Context, title, author string) string {
	bare := titles.WithoutAuthor(title, author)
	f, ok := s.Worker.Enricher().FindTitle(ctx, bare, author)
	if !ok || f.ISBN == "" {
		return ""
	}
	if _, same := enrich.ClosestTitle(bare, author, []enrich.Found{f}); !same {
		return ""
	}
	return f.ISBN
}

// saveISBNInCalibre writes a found ISBN into the book's Calibre entries (their identifiers), unless fixing
// Calibre by itself is off or the Content server isn't set up. Calibre's other identifiers are kept.
func (s *Server) saveISBNInCalibre(ctx context.Context, bookID int64, isbn string) {
	if s.Store.Setting(KeyNamesToCalibre) == "off" || s.Store.Setting(store.KeyCalibreSrvURL) == "" {
		return
	}
	cal := s.calibreClient()
	for _, cid := range s.Store.CalibreIDs(bookID) {
		have, err := cal.Details(ctx, []int{cid})
		d, ok := have[cid]
		if err != nil || !ok {
			continue
		}
		ids := map[string]string{}
		for k, v := range d.Identifiers {
			ids[k] = v
		}
		if ids["isbn"] != "" {
			continue // Calibre has one already
		}
		ids["isbn"] = isbn
		if _, err := cal.SetFields(ctx, cid, [][]any{{"identifiers", ids}}); err != nil {
			log.Printf("isbn to calibre: #%d: %v", cid, err)
		}
	}
}
