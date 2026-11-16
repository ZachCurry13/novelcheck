package api

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The name finder gives books that did not come from Calibre (Check a book,
// lists, events, Discover, Kindle and audiobook imports) their true title
// and author from Open Library: "Abby jiminez a married little Christmas"
// becomes "A Married Little Christmas" by Abby Jimenez. It only renames when
// it is sure it is the same book (enrich.BetterName), looks at each book
// once, and never touches a name a parent typed or a Calibre book (Calibre's
// titles are tidied through Library cleanup instead).

// trueName looks a book up and says what to call it, if anything else.
func (s *Server) trueName(ctx context.Context, title, author, isbn string) (enrich.Found, bool) {
	ec := s.Worker.Enricher()
	if isbn = enrich.CleanISBN(isbn); isbn != "" {
		if f, ok := ec.Find(ctx, isbn); ok {
			if better, ok := enrich.BetterName(title, author, true, f); ok {
				return better, true
			}
		}
	}
	for _, c := range ec.Search(ctx, title+" "+author, 3) {
		if better, ok := enrich.BetterName(title, author, false, c.Found()); ok {
			return better, true
		}
	}
	return enrich.Found{}, false
}

// fixNames checks up to limit books, about one a second, and says how many it renamed.
func (s *Server) fixNames(ctx context.Context, limit int) int {
	books, err := s.Store.NamesToCheck(limit)
	if err != nil {
		log.Printf("name finder: %v", err)
		return 0
	}
	fixed := 0
	for _, b := range books {
		if ctx.Err() != nil {
			break
		}
		if f, ok := s.trueName(ctx, b.Title, b.Author, b.ISBN); ok {
			if kept, err := s.Store.RenameBook(b.ID, f.Title, f.Author); err == nil {
				fixed++
				b.ID = kept
			} else if err != store.ErrFromCalibre {
				log.Printf("name finder: %q: %v", b.Title, err)
			}
		}
		s.Store.MarkName(b.ID, 1)
		time.Sleep(time.Second) // be gentle with Open Library
	}
	return fixed
}

// NameLoop runs the name finder a little after start and then once a day.
func (s *Server) NameLoop(ctx context.Context) {
	wait := 5 * time.Minute // before the cover finder (10 minutes), which then searches by the true title
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = 24 * time.Hour
		if n := s.fixNames(ctx, 500); n > 0 {
			s.Store.NotifyRoutine("names-fixed", fmt.Sprintf("Book names: %d books now have their true title and author from Open Library.", n), "")
		}
	}
}
