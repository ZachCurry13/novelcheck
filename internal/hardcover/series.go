package hardcover

import (
	"context"
	"encoding/json"
	"strings"
)

// SeriesEntry is one book of a series, as Hardcover lists it.
type SeriesEntry struct {
	Position float64
	Title    string
	Author   string
	CoverURL string
	Year     int
}

// SeriesBooks lists a series' books in order (one per position, the most read edition's book, no box sets
// or merged duplicates). name must be the series' name as Hardcover has it; the best match by name is used.
func (c *Client) SeriesBooks(ctx context.Context, name string) ([]SeriesEntry, error) {
	lit, _ := json.Marshal(name) // a quoted, escaped GraphQL string
	q := `query { series(where: {name: {_ilike: ` + string(lit) + `}}, limit: 5) { name
	  book_series(distinct_on: position, order_by: [{position: asc}, {book: {users_count: desc}}],
	    where: {position: {_is_null: false}, book: {compilation: {_eq: false}, canonical_id: {_is_null: true}}}) {
	    position book { title release_year cached_image cached_contributors } } } }`
	var d struct {
		Series []struct {
			Name  string `json:"name"`
			Books []struct {
				Position float64 `json:"position"`
				Book     struct {
					Title        string          `json:"title"`
					Year         int             `json:"release_year"`
					Image        json.RawMessage `json:"cached_image"`
					Contributors json.RawMessage `json:"cached_contributors"`
				} `json:"book"`
			} `json:"book_series"`
		} `json:"series"`
	}
	if err := c.do(ctx, q, &d); err != nil {
		return nil, err
	}
	best := -1
	for i, s := range d.Series { // the series with this exact name, else the one with the most books
		if strings.EqualFold(strings.TrimSpace(s.Name), strings.TrimSpace(name)) {
			best = i
			break
		}
		if best < 0 || len(s.Books) > len(d.Series[best].Books) {
			best = i
		}
	}
	if best < 0 {
		return nil, nil
	}
	var out []SeriesEntry
	for _, b := range d.Series[best].Books {
		if b.Position <= 0 || strings.TrimSpace(b.Book.Title) == "" {
			continue
		}
		e := SeriesEntry{Position: b.Position, Title: strings.TrimSpace(b.Book.Title), Year: b.Book.Year}
		var img struct {
			URL string `json:"url"`
		}
		if json.Unmarshal(b.Book.Image, &img) == nil {
			e.CoverURL = img.URL
		}
		var people []struct {
			Author struct {
				Name string `json:"name"`
			} `json:"author"`
		}
		if json.Unmarshal(b.Book.Contributors, &people) == nil && len(people) > 0 {
			e.Author = people[0].Author.Name
		}
		out = append(out, e)
	}
	return out, nil
}
