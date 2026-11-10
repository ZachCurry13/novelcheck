package hardcover

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// BookInfo is what Hardcover knows about a book: used to suggest corrections for a Calibre library.
type BookInfo struct {
	ID          int
	Title       string
	Authors     []string
	Series      string
	Position    float64 // 0 = not numbered
	Year        int
	Description string
	Tags        []string // the genres Hardcover's readers gave it, most used first
	Image       string
	ISBN13      string
	ISBN10      string
	Pages       int
}

// bookFields is what is asked about a book (the same in every lookup).
const bookFields = `id title release_year description cached_tags cached_image
  contributions(limit: 3) { author { name } }
  book_series(order_by: {position: asc}, limit: 1, where: {position: {_is_null: false}}) { position series { name } }`

type rawBook struct {
	ID            int             `json:"id"`
	Title         string          `json:"title"`
	Year          int             `json:"release_year"`
	Description   string          `json:"description"`
	Tags          json.RawMessage `json:"cached_tags"`
	Image         json.RawMessage `json:"cached_image"`
	Contributions []struct {
		Author struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"contributions"`
	BookSeries []struct {
		Position float64 `json:"position"`
		Series   struct {
			Name string `json:"name"`
		} `json:"series"`
	} `json:"book_series"`
	Editions []struct {
		Pages  int    `json:"pages"`
		ISBN13 string `json:"isbn_13"`
		ISBN10 string `json:"isbn_10"`
	} `json:"editions"`
}

func (r rawBook) info() BookInfo {
	b := BookInfo{ID: r.ID, Title: strings.TrimSpace(r.Title), Year: r.Year, Description: strings.TrimSpace(r.Description)}
	for _, c := range r.Contributions {
		if n := strings.TrimSpace(c.Author.Name); n != "" {
			b.Authors = append(b.Authors, n)
		}
	}
	if len(r.BookSeries) > 0 {
		b.Series, b.Position = strings.TrimSpace(r.BookSeries[0].Series.Name), r.BookSeries[0].Position
	}
	var img struct {
		URL string `json:"url"`
	}
	if json.Unmarshal(r.Image, &img) == nil {
		b.Image = img.URL
	}
	var tags map[string][]struct {
		Tag string `json:"tag"`
	}
	if json.Unmarshal(r.Tags, &tags) == nil {
		for _, t := range tags["Genre"] {
			if len(b.Tags) < 6 && strings.TrimSpace(t.Tag) != "" {
				b.Tags = append(b.Tags, strings.TrimSpace(t.Tag))
			}
		}
	}
	if len(r.Editions) > 0 {
		b.ISBN13, b.ISBN10, b.Pages = r.Editions[0].ISBN13, r.Editions[0].ISBN10, r.Editions[0].Pages
	}
	return b
}

// InfoByISBN looks up many ISBNs in one call (up to 25 at a time) and returns each one's book, keyed by the
// ISBN that was asked about.
func (c *Client) InfoByISBN(ctx context.Context, isbns []string) (map[string]BookInfo, error) {
	var clean []string
	for _, i := range isbns {
		if d := digits(i); len(d) == 10 || len(d) == 13 {
			clean = append(clean, `"`+d+`"`)
		}
	}
	out := map[string]BookInfo{}
	if len(clean) == 0 {
		return out, nil
	}
	if len(clean) > 25 {
		clean = clean[:25]
	}
	list := strings.Join(clean, ", ")
	var d struct {
		Editions []struct {
			ISBN13 string  `json:"isbn_13"`
			ISBN10 string  `json:"isbn_10"`
			Pages  int     `json:"pages"`
			Book   rawBook `json:"book"`
		} `json:"editions"`
	}
	q := fmt.Sprintf(`query { editions(where: {_or: [{isbn_13: {_in: [%s]}}, {isbn_10: {_in: [%s]}}]}, limit: 50) { isbn_13 isbn_10 pages book { %s } } }`, list, list, bookFields)
	if err := c.do(ctx, q, &d); err != nil {
		return nil, err
	}
	for _, e := range d.Editions {
		b := e.Book.info()
		b.ISBN13, b.ISBN10, b.Pages = e.ISBN13, e.ISBN10, e.Pages
		for _, k := range []string{e.ISBN13, e.ISBN10} {
			if k != "" {
				out[k] = b
			}
		}
	}
	return out, nil
}

// InfoByTitle finds a book by title and author through Hardcover's search, accepting only an exact title and
// author match (so a look-alike never provides "corrections").
func (c *Client) InfoByTitle(ctx context.Context, title, author string) (BookInfo, bool, error) {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(author) == "" {
		return BookInfo{}, false, nil
	}
	lit, _ := json.Marshal(strings.TrimSpace(title + " " + author))
	var s struct {
		Search struct {
			IDs []json.RawMessage `json:"ids"`
		} `json:"search"`
	}
	if err := c.do(ctx, `query { search(query: `+string(lit)+`, query_type: "Book", per_page: 5, page: 1) { ids } }`, &s); err != nil {
		return BookInfo{}, false, err
	}
	var ids []string
	for _, raw := range s.Search.IDs {
		if n, err := strconv.Atoi(strings.Trim(string(raw), `"`)); err == nil {
			ids = append(ids, strconv.Itoa(n))
		}
	}
	if len(ids) == 0 {
		return BookInfo{}, false, nil
	}
	var d struct {
		Books []rawBook `json:"books"`
	}
	q := fmt.Sprintf(`query { books(where: {id: {_in: [%s]}}) { %s editions(order_by: {users_count: desc_nulls_last}, limit: 1) { pages isbn_13 isbn_10 } } }`, strings.Join(ids, ", "), bookFields)
	if err := c.do(ctx, q, &d); err != nil {
		return BookInfo{}, false, err
	}
	want := store.NormKey(title, author)
	for _, r := range d.Books {
		b := r.info()
		for _, a := range b.Authors {
			if store.NormKey(b.Title, a) == want {
				return b, true, nil
			}
		}
	}
	return BookInfo{}, false, nil
}
