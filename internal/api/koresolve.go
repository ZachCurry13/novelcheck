package api

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/enrich"
)

// KOReader books that match nothing in the library are looked up in Open
// Library (free), so they get a cover and a real title and author. They are
// kept like "Check a book" results ("Looked up"), not as part of the library.

var (
	uuidish      = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}`)
	notBookTitle = regexp.MustCompile(`(?i)thumbnail|quickstart|^error:|\.(epub|pdf|mobi|azw3?)$`)
)

const resolvePerRun = 15

// looksLikeBook skips things that are not worth looking up: file names,
// IDs, KOReader's own guide.
func looksLikeBook(title string) bool {
	t := strings.TrimSpace(title)
	return len(t) >= 4 && !uuidish.MatchString(t) && !notBookTitle.MatchString(t) && strings.ContainsAny(t, " abcdefghijklmnopqrstuvwxyz")
}

// sameTitle says whether two titles are the same book: equal, or one inside
// the other (a subtitle or series label), once reduced to letters and digits.
func sameTitle(a, b string) bool {
	squash := func(s string) string {
		var out strings.Builder
		for _, r := range strings.ToLower(s) {
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
				out.WriteRune(r)
			}
		}
		return out.String()
	}
	x, y := squash(a), squash(b)
	if x == "" || y == "" {
		return false
	}
	return x == y || (len(x) >= 6 && strings.Contains(y, x)) || (len(y) >= 6 && strings.Contains(x, y))
}

// resolveUnmatched looks up someone's unmatched KOReader books in Open
// Library, at most once an hour and a few at a time. It runs in the background.
func (s *Server) resolveUnmatched(userID int64) {
	if s.Worker == nil {
		return
	}
	if at, ok := s.resolvedAt.Load(userID); ok && time.Since(at.(time.Time)) < time.Hour {
		return
	}
	s.resolvedAt.Store(userID, time.Now())
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		ec := s.Worker.Enricher()
		for _, b := range s.Store.UnmatchedKOReader(userID, resolvePerRun) {
			if !looksLikeBook(b.Title) {
				continue
			}
			var found enrich.Found
			var ok bool
			if a := strings.ToLower(strings.TrimSpace(b.Authors)); a == "" || a == "n/a" || a == "owner" || a == "unknown" {
				var choices []enrich.Choice
				if choices = ec.Search(ctx, b.Title, 3); len(choices) > 0 && enrich.Sure(b.Title, choices) {
					found, ok = choices[0].Found(), true
				}
			} else if f, got := ec.FindTitle(ctx, b.Title, b.Authors); got && sameTitle(f.Title, b.Title) {
				found, ok = f, true
			}
			if !ok || found.Title == "" {
				continue
			}
			id := s.Store.MatchBook(found.Title, found.Author) // in the library under its catalogued name?
			if id == 0 {
				var err error
				if id, err = s.saveLookup(found); err != nil {
					continue
				}
			}
			s.Store.SetKOReaderBook(userID, b, id)
			time.Sleep(400 * time.Millisecond) // be gentle with Open Library
		}
	}()
}
