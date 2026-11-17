package api

import (
	"context"
	"log"
	"time"

	"github.com/zachcurry13/novelcheck/internal/hardcover"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Hardcover ratings for every book (store/hcratings.go): looked up in the background with a family member's
// Hardcover connection, by ISBN 25 at a time, then by title and author (a messy title by the true one), about
// one request a second. A book Hardcover doesn't know is looked at again after a month.

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
	found := 0
	keep := func(b store.RatingToCheck, info hardcover.BookInfo) {
		s.Store.SaveHCRating(b.ID, info.Rating, info.Ratings)
		if info.Rating > 0 {
			found++
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
		info, ok, err := hc.InfoByTitle(ctx, b.Title, b.Author)
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

// RatingLoop fills ratings 12 minutes after start, then every 6 hours, 400 books a round.
func (s *Server) RatingLoop(ctx context.Context) {
	wait := 12 * time.Minute
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = 6 * time.Hour
		if n := s.FillRatings(ctx, 400); n > 0 {
			log.Printf("hardcover ratings: %d books rated", n)
		}
	}
}
