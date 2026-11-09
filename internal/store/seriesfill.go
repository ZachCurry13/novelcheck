package store

import (
	"sort"
	"strings"
)

// SeriesFill is a book of a series that someone else's list says exists (from Hardcover).
type SeriesFill struct {
	Position float64
	Title    string
	Author   string
	CoverURL string
}

// SeriesAuthor is the author most of a series' books have here (what the missing ones are filed under).
func (s *Store) SeriesAuthor(series string) string {
	var a string
	_ = s.DB.Get(&a, `SELECT author FROM books WHERE LOWER(series) = LOWER(?) AND author != '' GROUP BY author ORDER BY COUNT(*) DESC LIMIT 1`, series)
	return a
}

// AddSeriesBooks adds the books of a series the family does not have, as looked-up books (not in a library):
// each takes its place in the series, so it shows on the Series page with its number, can be requested, and
// can be found at a Libby library. A book the family already has (under the same number, or under a title
// that matches) is left alone. It says how many were added.
func (s *Store) AddSeriesBooks(series, author string, entries []SeriesFill) (int, error) {
	catID, err := s.EnsureCatalog(LookedUpCatalog, "custom")
	if err != nil {
		return 0, err
	}
	have := map[float64]bool{}
	var idx []float64
	_ = s.DB.Select(&idx, `SELECT series_index FROM books WHERE LOWER(series) = LOWER(?) AND series_index > 0`, series)
	for _, i := range idx {
		have[i] = true
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Position < entries[j].Position })
	added := 0
	for _, e := range entries {
		if have[e.Position] || strings.TrimSpace(e.Title) == "" {
			continue
		}
		who := author
		if who == "" {
			who = e.Author
		}
		if id := s.MatchBook(e.Title, who); id > 0 { // already a book here: only its place in the series
			_, _ = s.DB.Exec(`UPDATE books SET series = CASE WHEN series = '' THEN ? ELSE series END,
				series_index = CASE WHEN series_index = 0 THEN ? ELSE series_index END WHERE id = ?`, series, e.Position, id)
			continue
		}
		id, err := s.UpsertBook(e.Title, who, "", "")
		if err != nil {
			continue
		}
		if err := s.AddCopy(catID, id, "lookup:"+e.Title, "list", ""); err != nil {
			return added, err
		}
		_, _ = s.DB.Exec(`UPDATE books SET series = ?, series_index = ?, cover_url = CASE WHEN ? != '' THEN ? ELSE cover_url END WHERE id = ?`,
			series, e.Position, e.CoverURL, e.CoverURL, id)
		have[e.Position] = true
		added++
	}
	return added, nil
}

// BookCoverURL is a cover address kept with a book (from the series lookup), or "".
func (s *Store) BookCoverURL(id int64) string {
	var u string
	_ = s.DB.Get(&u, `SELECT cover_url FROM books WHERE id = ?`, id)
	return u
}
