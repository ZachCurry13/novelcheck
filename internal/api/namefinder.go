package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/titles"
)

// The name finder gives books that did not come from Calibre (Check a book,
// lists, events, Discover, Kindle and audiobook imports) their true title
// and author from Open Library: "Abby jiminez a married little Christmas"
// becomes "A Married Little Christmas" by Abby Jimenez. It only renames when
// it is sure it is the same book (enrich.BetterName), looks at each book
// once, and never touches a name a parent typed or a Calibre book (Calibre's
// titles are tidied through Library cleanup instead).

// closestTitle finds a messy title among the author's own books on Open Library ("Unnatural.Exprosure" is
// Unnatural Exposure); "" when no single book is clearly meant or the title is already right.
func (s *Server) closestTitle(ctx context.Context, title, author string) string {
	bare := titles.WithoutAuthor(title, author)
	f, ok := enrich.ClosestTitle(bare, author, s.Worker.Enricher().AuthorTitles(ctx, author))
	if !ok || f.Title == strings.TrimSpace(title) {
		return ""
	}
	return f.Title
}

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
	if t := s.closestTitle(ctx, title, author); t != "" { // a typo or dots: among the author's books
		return enrich.Found{Title: t, Author: author}, true
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
	fixed, saved := 0, 0
	defer func() {
		if saved > 0 {
			log.Printf("name finder: saved %d true titles in Calibre", saved)
		}
	}()
	for _, b := range books {
		if ctx.Err() != nil {
			break
		}
		if b.Calibre {
			// Calibre's title would come back at the next sync, so the true title is a suggestion to save in
			// Calibre (Tidy titles). Only a title that is off (a typo, dots, odd capitals) gets one, so a
			// title Open Library merely writes longer is left alone.
			if t := s.closestTitle(ctx, b.Title, b.Author); t != "" {
				s.Store.SuggestTitle(b.ID, t)
				if s.saveTitleInCalibre(ctx, b.ID, t) {
					saved++
				}
				fixed++
			}
		} else if f, ok := s.trueName(ctx, b.Title, b.Author, b.ISBN); ok {
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

const namesVersionKey, namesVersion = "names_finder_version", "2"

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
		if s.Store.Setting(namesVersionKey) != namesVersion { // the finder learned more (Calibre books, typos): look again
			s.Store.RecheckNames()
			_ = s.Store.SetSetting(namesVersionKey, namesVersion)
		}
		if n := s.fixNames(ctx, 2000); n > 0 {
			s.Store.NotifyRoutine("names-fixed", fmt.Sprintf("Book names: %d books have their true title from Open Library. Calibre books are waiting under Admin → Library → Tidy titles in Calibre.", n), "#/admin?tab=cleanup")
		}
	}
}

// KeyNamesToCalibre turns off saving the name finder's titles in Calibre by itself ("off"); they are then only
// listed under Tidy titles in Calibre.
const KeyNamesToCalibre = "calibre_names_auto"

// saveTitleInCalibre saves a true title the name finder is sure of in Calibre (through the Content server, the
// same way Tidy titles does, checking the book first), unless that is turned off or not set up. It says
// whether Calibre took it.
func (s *Server) saveTitleInCalibre(ctx context.Context, bookID int64, title string) bool {
	if s.Store.Setting(KeyNamesToCalibre) == "off" || s.Store.Setting(store.KeyCalibreSrvURL) == "" {
		return false
	}
	b, err := s.Store.BookByID(bookID, nil)
	if err != nil {
		return false
	}
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
	fixed, skipped, ok := s.pushTitles(rec, r, []titlePush{{Book: bookID, Title: title, Series: b.Series, Index: b.SeriesIndex}})
	if !ok || fixed == 0 {
		log.Printf("name finder: “%s” → “%s” not saved in Calibre: %s%s", b.Title, title, strings.TrimSpace(rec.Body.String()), strings.Join(skipped, "; "))
		return false
	}
	return true
}
