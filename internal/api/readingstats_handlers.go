package api

import (
	"net/http"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

const maxStatHours = 60000 // about 7 years of every hour of the day

// handleMyReadingStats sends someone's reading for the Reading stats page:
// the books they've opened and their reading by the hour (UTC). The browser
// works out days, weeks and streaks in its own time zone.
func (s *Server) handleMyReadingStats(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	s.Store.RematchUnmatched(u.ID)
	s.backfillHours(u.ID)
	books, _ := s.Store.KOReaderBooks(u.ID, 5000)
	hours, _ := s.Store.ReadingHours(u.ID, maxStatHours)
	type book struct {
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
	var ids []int64
	for _, b := range books {
		if b.BookID != nil {
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
	outBooks := make([]book, 0, len(books))
	for i, b := range books {
		index[[3]string{b.Title, b.Authors, b.MD5}] = i
		x := book{Title: b.Title, Author: b.Authors, BookID: b.BookID, Pages: b.Pages, Percent: b.Percent, ReadSeconds: b.ReadSeconds,
			LastOpen: b.LastOpen, Highlights: b.Highlights, Notes: b.Notes}
		if b.BookID != nil {
			if br, ok := briefs[*b.BookID]; ok {
				x.Kind, x.Genres, x.Series, x.SeriesTotal = br.Kind, br.GenreList(), br.Series, seriesTotals[br.Series]
				if u.Role != "restricted" {
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
	if _, at := s.statsPath(u.ID); !at.IsZero() {
		synced = at.Unix()
	}
	writeJSON(w, http.StatusOK, map[string]any{"books": outBooks, "hours": rows, "synced_at": synced})
}

// backfillHours reads the statistics file already in someone's folder: once
// per run (a new version may keep more of it, like highlights), and again,
// at most every ten minutes, while no hours are stored. It was uploaded
// before NovelCheck kept the hours, and KOReader only uploads when something changed.
func (s *Server) backfillHours(userID int64) {
	has := s.Store.HasReadingHours(userID)
	if _, done := s.hoursSeen.Load(userID); done && has {
		return
	}
	if !has {
		if at, ok := s.hoursTried.Load(userID); ok && time.Since(at.(time.Time)) < 10*time.Minute {
			return
		}
	}
	s.hoursSeen.Store(userID, true)
	if path, _ := s.statsPath(userID); path != "" {
		s.importKOStats(userID)
	}
	if !s.Store.HasReadingHours(userID) {
		s.hoursTried.Store(userID, time.Now())
	}
}
