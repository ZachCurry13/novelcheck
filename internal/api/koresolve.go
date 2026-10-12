package api

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/titles"
)

// Books on a person's e-readers that match nothing in the library are looked
// up in Open Library (free), so they get a cover and a real title and author.
// They are kept like "Check a book" results ("Looked up"), not as part of the
// library.

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

func junkAuthor(a string) bool {
	switch strings.ToLower(strings.TrimSpace(a)) {
	case "", "n/a", "owner", "unknown", "anonymous":
		return true
	}
	return false
}

// lookupBook asks Open Library about a book and only answers when it is sure.
func lookupBook(ctx context.Context, ec *enrich.Client, title, author string) (enrich.Found, bool) {
	if !looksLikeBook(title) {
		return enrich.Found{}, false
	}
	if junkAuthor(author) {
		if choices := ec.Search(ctx, title, 3); len(choices) > 0 && enrich.Sure(title, choices) {
			return choices[0].Found(), true
		}
		return enrich.Found{}, false
	}
	if f, ok := ec.FindTitle(ctx, title, author); ok && f.Title != "" && sameTitle(f.Title, title) {
		return f, true
	}
	return enrich.Found{}, false
}

// resolveUnmatched looks up someone's unmatched books (from KOReader's
// statistics and the files on their devices) in Open Library, at most once an
// hour and a few at a time. It runs in the background.
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
		left := resolvePerRun
		link := func(f enrich.Found) (int64, bool) {
			id := s.Store.MatchBook(f.Title, f.Author) // in the library under its catalogued name?
			if id == 0 {
				var err error
				if id, err = s.saveLookup(f); err != nil {
					return 0, false
				}
			}
			time.Sleep(400 * time.Millisecond) // be gentle with Open Library
			return id, true
		}
		for _, b := range s.Store.UnmatchedKOReader(userID, left) {
			if f, ok := lookupBook(ctx, ec, b.Title, b.Authors); ok {
				if id, ok := link(f); ok {
					s.Store.SetKOReaderBook(userID, b, id)
				}
			}
			left--
		}
		for _, b := range s.Store.UnmatchedDevice(userID, max(left, 0)) {
			title, author := b.Title, b.Author
			if author == "" { // only a file name: take its best reading
				if gs := titles.FromFilename(b.Filename); len(gs) > 0 {
					title, author = gs[0].Title, gs[0].Author
					if author != "" && !looksLikeBook(title) {
						title, author = gs[len(gs)-1].Title, ""
					}
				}
			}
			if f, ok := lookupBook(ctx, ec, title, author); ok {
				if id, ok := link(f); ok {
					s.Store.SetDeviceBook(userID, b, id)
				}
			}
		}
	}()
}
