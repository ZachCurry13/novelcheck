package api

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/calibresrv"
	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/titles"
)

// isbnBestGuess offers an ISBN for a Calibre book with none that Hardcover doesn't know: Open Library's first
// book for its title (without a subtitle or series note) by the same author. It may be another edition, so it is
// only a suggestion, marked "best guess", that the admin ticks or not; nothing is written by itself.
func (s *Server) isbnBestGuess(ctx context.Context, d calibresrv.Details, bookID int64, calibreID int) (store.HCSuggestion, bool) {
	if d.Identifiers["isbn"] != "" || len(d.Authors) == 0 {
		return store.HCSuggestion{}, false
	}
	t := titles.WithoutAuthor(d.Title, d.Authors[0])
	if i := strings.IndexAny(t, ":("); i > 0 {
		t = strings.TrimSpace(t[:i])
	}
	f, ok := s.Worker.Enricher().FindTitle(ctx, t, d.Authors[0])
	if t == "" || !ok || f.ISBN == "" || !sameSurname(f.Author, d.Authors[0]) {
		return store.HCSuggestion{}, false
	}
	ids := map[string]string{"isbn": f.ISBN}
	for k, v := range d.Identifiers {
		if k != "isbn" {
			ids[k] = v
		}
	}
	payload, _ := json.Marshal(ids)
	return store.HCSuggestion{CalibreID: calibreID, BookID: bookID, Title: d.Title, Field: "identifiers", Current: "(no ISBN)",
		Shown: "isbn:" + f.ISBN + " (Open Library: “" + f.Title + "”)", Payload: string(payload), Confidence: "best guess"}, true
}

// sameSurname says whether two author names end in the same last name ("Lemony Snicket", "Snicket, Lemony").
func sameSurname(a, b string) bool {
	last := func(n string) string {
		if i := strings.Index(n, ","); i > 0 {
			n = n[:i]
		} else if f := strings.Fields(n); len(f) > 0 {
			n = f[len(f)-1]
		}
		return strings.ToLower(strings.TrimSpace(n))
	}
	return last(a) != "" && last(a) == last(b)
}
