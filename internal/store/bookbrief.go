package store

import (
	"strings"

	"github.com/jmoiron/sqlx"
)

// BookBrief is the little about a book that Reading stats charts need.
type BookBrief struct {
	ID     int64  `db:"id"`
	Kind   string `db:"kind"` // fiction, nonfiction or ""
	Genres string `db:"genres"`
	Spice  *int   `db:"spice_level"`
	Series string `db:"series"`
}

// GenreList splits the ",fantasy,romance," form.
func (b BookBrief) GenreList() []string {
	var out []string
	for _, g := range strings.Split(b.Genres, ",") {
		if g = strings.TrimSpace(g); g != "" {
			out = append(out, g)
		}
	}
	return out
}

// BookBriefs fetches those facts for some books at once.
func (s *Store) BookBriefs(ids []int64) map[int64]BookBrief {
	out := map[int64]BookBrief{}
	for len(ids) > 0 {
		n := min(len(ids), 400)
		query, args, err := sqlx.In(`SELECT id, kind, genres, spice_level, series FROM books WHERE id IN (?)`, ids[:n])
		if err != nil {
			return out
		}
		var rows []BookBrief
		if s.DB.Select(&rows, s.DB.Rebind(query), args...) == nil {
			for _, r := range rows {
				out[r.ID] = r
			}
		}
		ids = ids[n:]
	}
	return out
}

// SeriesCounts is how many library books each named series has.
func (s *Store) SeriesCounts(series []string) map[string]int {
	out := map[string]int{}
	for len(series) > 0 {
		n := min(len(series), 300)
		query, args, err := sqlx.In(`SELECT b.series AS name, COUNT(*) AS n FROM books b WHERE b.series IN (?) AND `+ownedCond+` GROUP BY b.series`, series[:n])
		if err == nil {
			var rows []struct {
				Name string `db:"name"`
				N    int    `db:"n"`
			}
			if s.DB.Select(&rows, s.DB.Rebind(query), args...) == nil {
				for _, r := range rows {
					out[r.Name] = r.N
				}
			}
		}
		series = series[n:]
	}
	return out
}
