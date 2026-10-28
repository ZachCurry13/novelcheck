package api

import (
	"github.com/zachcurry13/novelcheck/internal/store"
)

const maxStatHours = 60000 // about 7 years of every hour of the day

type statBook struct {
	Title       string   `json:"title"`
	Author      string   `json:"author"`
	BookID      *int64   `json:"book_id"`
	Pages       int      `json:"pages"`
	Percent     float64  `json:"percent"`
	ReadSeconds int64    `json:"read_seconds"`
	LastOpen    int64    `json:"last_open"`
	Highlights  int      `json:"highlights"`
	Notes       int      `json:"notes"`
	Kind        string   `json:"kind,omitempty"` // fiction or nonfiction, when known
	Genres      []string `json:"genres,omitempty"`
	Spice       *int     `json:"spice,omitempty"` // pepper level (not for kids' accounts)
	Series      string   `json:"series,omitempty"`
	SeriesTotal int      `json:"series_total,omitempty"` // books in that series in the library
}

// readingStatsFor is owner's reading for the Reading stats page, as viewer may
// see it: the books they've opened and their reading by the hour (UTC); the
// browser works out days, weeks and streaks in its own time zone. When viewer
// is someone else, only books viewer is allowed to see are included (kids'
// content rules apply), books that are not in the library (personal files) and
// highlights and notes are left out, and kids' accounts never see pepper levels.
func (s *Server) readingStatsFor(owner, viewer *store.User) map[string]any {
	own := owner.ID == viewer.ID
	books, _ := s.Store.KOReaderBooks(owner.ID, 5000)
	hours, _ := s.Store.ReadingHours(owner.ID, maxStatHours)

	visible := make([]bool, len(books))
	var ids []int64
	for i, b := range books {
		switch {
		case own:
			visible[i] = true
		case b.BookID != nil:
			if _, err := s.Store.BookByID(*b.BookID, viewer); err == nil {
				visible[i] = true
			}
		}
		if visible[i] && b.BookID != nil {
			ids = append(ids, *b.BookID)
		}
	}
	briefs := s.Store.BookBriefs(ids)
	var seriesNames []string
	seenSeries := map[string]bool{}
	for _, br := range briefs {
		if br.Series != "" && !seenSeries[br.Series] {
			seenSeries[br.Series] = true
			seriesNames = append(seriesNames, br.Series)
		}
	}
	seriesTotals := s.Store.SeriesCounts(seriesNames)

	index := map[[3]string]int{}
	outBooks := make([]statBook, 0, len(books))
	for i, b := range books {
		if !visible[i] {
			continue
		}
		index[[3]string{b.Title, b.Authors, b.MD5}] = len(outBooks)
		x := statBook{Title: b.Title, Author: b.Authors, BookID: b.BookID, Pages: b.Pages, Percent: b.Percent, ReadSeconds: b.ReadSeconds, LastOpen: b.LastOpen}
		if own { // highlights and notes are personal
			x.Highlights, x.Notes = b.Highlights, b.Notes
		}
		if b.BookID != nil {
			if br, ok := briefs[*b.BookID]; ok {
				x.Kind, x.Genres, x.Series, x.SeriesTotal = br.Kind, br.GenreList(), br.Series, seriesTotals[br.Series]
				if viewer.Role != store.RoleRestricted {
					x.Spice = br.Spice
				}
			}
		}
		outBooks = append(outBooks, x)
	}
	rows := make([][4]int64, 0, len(hours)) // [hour, book index, seconds, pages]
	for _, h := range hours {
		if i, ok := index[[3]string{h.Title, h.Authors, h.MD5}]; ok {
			rows = append(rows, [4]int64{h.Hour, int64(i), h.Seconds, int64(h.Pages)})
		}
	}
	var synced int64
	if _, at := s.statsPath(owner.ID); !at.IsZero() {
		synced = at.Unix()
	}
	return map[string]any{"books": outBooks, "hours": rows, "synced_at": synced}
}
