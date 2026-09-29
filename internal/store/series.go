package store

// Series: the family's books grouped by the series Calibre gives them, in
// order, with what the viewer has read and what comes next.

import (
	"math"
	"strconv"
	"strings"
)

// SeriesSummary is one series as someone sees it.
type SeriesSummary struct {
	Name    string  `db:"name" json:"name"`
	Books   int     `db:"books" json:"books"`
	Read    int     `db:"read" json:"read"`       // the viewer finished them
	Reading int     `db:"reading" json:"reading"` // the viewer is reading them
	Top     float64 `db:"top" json:"top"`         // the highest number the family has
	IDs     string  `db:"ids" json:"-"`
	Covers  []int64 `json:"covers"` // the first few books' ids, for covers
}

// SeriesBook is a book of a series with where the viewer is with it.
type SeriesBook struct {
	Book
	MyStatus string `db:"my_status" json:"my_status"` // "", queued, reading or finished
}

// seriesWhere is the series' books the family has that the viewer may see.
func seriesWhere(u *User) (string, []any) {
	vis, args := visibilityClause(u)
	return ` AND b.series != '' AND ` + ownedCond + vis, args
}

// SeriesList returns every series the viewer can see, by name.
func (s *Store) SeriesList(u *User) ([]SeriesSummary, error) {
	where, args := seriesWhere(u)
	uid := viewerID(u)
	out := []SeriesSummary{}
	err := s.DB.Select(&out, `SELECT MIN(b.series) AS name, COUNT(*) AS books,
		SUM(EXISTS (SELECT 1 FROM queue_items q WHERE q.book_id = b.id AND q.user_id = ? AND q.status = 'finished')) AS read,
		SUM(EXISTS (SELECT 1 FROM queue_items q WHERE q.book_id = b.id AND q.user_id = ? AND q.status = 'reading')) AS reading,
		MAX(b.series_index) AS top, GROUP_CONCAT(b.id) AS ids
		FROM (SELECT b.* FROM books b WHERE 1=1`+where+` ORDER BY b.series_index, b.id) b
		GROUP BY LOWER(b.series) ORDER BY sort_title(MIN(b.series))`, append([]any{uid, uid}, args...)...)
	for i := range out {
		for _, f := range strings.Split(out[i].IDs, ",") {
			if id, err := strconv.ParseInt(f, 10, 64); err == nil && len(out[i].Covers) < 3 {
				out[i].Covers = append(out[i].Covers, id)
			}
		}
		if out[i].Covers == nil {
			out[i].Covers = []int64{}
		}
	}
	return out, err
}

// SeriesBooks returns a series' books in order, with the viewer's status.
func (s *Store) SeriesBooks(name string, u *User) ([]SeriesBook, error) {
	where, args := seriesWhere(u)
	out := []SeriesBook{}
	err := s.DB.Select(&out, `SELECT `+bookCols+derivedCols+`, '' AS catalogs,
		COALESCE((SELECT q.status FROM queue_items q WHERE q.book_id = b.id AND q.user_id = ?), '') AS my_status
		FROM books b WHERE LOWER(b.series) = LOWER(?)`+where+` ORDER BY b.series_index = 0, b.series_index, sort_title(b.title)`,
		append([]any{viewerID(u), strings.TrimSpace(name)}, args...)...)
	return out, err
}

// SeriesGaps are the whole numbers from 1 to the highest one the family
// has that no book of the series carries (books the family doesn't have).
func SeriesGaps(books []SeriesBook) []int {
	have := map[int]bool{}
	top := 0
	for _, b := range books {
		if b.SeriesIndex > 0 {
			n := int(math.Floor(b.SeriesIndex))
			have[n] = true
			top = max(top, n)
		}
	}
	gaps := []int{}
	for n := 1; n < top && len(gaps) < 50; n++ {
		if !have[n] {
			gaps = append(gaps, n)
		}
	}
	return gaps
}

// NextInSeries is the book to read next: the first one after the furthest
// the viewer has finished or is reading that they haven't finished; the
// first book if they haven't started. 0 when there's none.
func NextInSeries(books []SeriesBook) int64 {
	after := -1
	for i, b := range books {
		if b.MyStatus == "finished" || b.MyStatus == "reading" {
			after = i
		}
	}
	for _, b := range books[after+1:] {
		if b.MyStatus != "finished" {
			return b.ID
		}
	}
	return 0
}
