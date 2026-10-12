package api

import (
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

const maxStatHours = 60000 // about 7 years of every hour of the day

// handleMyReadingStats sends someone's reading for the Reading stats page:
// the books they've opened and their reading by the hour (UTC). The browser
// works out days, weeks and streaks in its own time zone.
func (s *Server) handleMyReadingStats(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	s.Store.RematchUnmatched(u.ID)
	books, _ := s.Store.KOReaderBooks(u.ID, 5000)
	hours, _ := s.Store.ReadingHours(u.ID, maxStatHours)
	type book struct {
		Title       string  `json:"title"`
		Author      string  `json:"author"`
		BookID      *int64  `json:"book_id"`
		Pages       int     `json:"pages"`
		Percent     float64 `json:"percent"`
		ReadSeconds int64   `json:"read_seconds"`
		LastOpen    int64   `json:"last_open"`
	}
	index := map[[3]string]int{}
	outBooks := make([]book, 0, len(books))
	for i, b := range books {
		index[[3]string{b.Title, b.Authors, b.MD5}] = i
		outBooks = append(outBooks, book{b.Title, b.Authors, b.BookID, b.Pages, b.Percent, b.ReadSeconds, b.LastOpen})
	}
	rows := make([][4]int64, 0, len(hours)) // [hour, book index, seconds, pages]
	for _, h := range hours {
		if i, ok := index[[3]string{h.Title, h.Authors, h.MD5}]; ok {
			rows = append(rows, [4]int64{h.Hour, int64(i), h.Seconds, int64(h.Pages)})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"books": outBooks, "hours": rows})
}
