package store

// Browsing the library by author, series, genre and fiction/nonfiction, and
// the choices (with counts) the filters offer.

import (
	"sort"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/genres"
)

// browseConds adds the author, series, genre and fiction/nonfiction filters.
func browseConds(f BookFilter, where []string, args []any) ([]string, []any) {
	if a := strings.TrimSpace(f.Author); a != "" {
		where = append(where, "b.author LIKE ?")
		args = append(args, "%"+a+"%")
	}
	if s := strings.TrimSpace(f.Series); s != "" {
		where = append(where, "b.series LIKE ?")
		args = append(args, "%"+s+"%")
	}
	if genres.Valid(f.Genre) {
		where = append(where, "b.genres LIKE ?")
		args = append(args, "%,"+f.Genre+",%")
	}
	switch f.Kind {
	case "fiction", "nonfiction":
		where = append(where, "b.kind = ?")
		args = append(args, f.Kind)
	case "unknown":
		where = append(where, "b.kind = ''")
	}
	return where, args
}

// Facet is one choice in a filter, with how many books it has.
type Facet struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// Facets are the author, series, genre and fiction/nonfiction choices for
// the books viewer may see, most books first (genres in their usual order).
type Facets struct {
	Genres  []Facet  `json:"genres"`
	Kinds   []Facet  `json:"kinds"`
	Authors []string `json:"authors"`
	Series  []string `json:"series"`
}

func (s *Store) Facets(viewer *User) (Facets, error) {
	where, args := visibilityClause(viewer)
	var rows []struct {
		Author string `db:"author"`
		Series string `db:"series"`
		Genres string `db:"genres"`
		Kind   string `db:"kind"`
	}
	if err := s.DB.Select(&rows, `SELECT b.author, b.series, b.genres, b.kind FROM books b
		WHERE EXISTS (SELECT 1 FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id WHERE cb.book_id = b.id AND c.name != ?)`+where,
		append([]any{LookedUpCatalog}, args...)...); err != nil {
		return Facets{}, err
	}
	genreN, kindN, authorN, seriesN := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	for _, r := range rows {
		for _, g := range genres.Split(r.Genres) {
			genreN[g]++
		}
		kindN[r.Kind]++
		for _, a := range strings.Split(r.Author, " & ") {
			if a = strings.TrimSpace(a); a != "" {
				authorN[a]++
			}
		}
		if r.Series != "" {
			seriesN[r.Series]++
		}
	}
	var f Facets
	for _, g := range genres.All {
		if n := genreN[g.Key]; n > 0 {
			f.Genres = append(f.Genres, Facet{g.Key, g.Label, n})
		}
	}
	f.Kinds = []Facet{{"fiction", "Fiction", kindN["fiction"]}, {"nonfiction", "Nonfiction", kindN["nonfiction"]}, {"unknown", "Not sorted yet", kindN[""]}}
	f.Authors, f.Series = byCount(authorN, 3000), byCount(seriesN, 3000)
	return f, nil
}

// byCount lists names, most books first, then alphabetically.
func byCount(m map[string]int, limit int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if m[out[i]] != m[out[j]] {
			return m[out[i]] > m[out[j]]
		}
		return out[i] < out[j]
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
