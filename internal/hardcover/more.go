package hardcover

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// More is what the book page shows from Hardcover beyond the rating: the audiobook edition (length, narrators,
// ASIN) and the books Hardcover's readers find similar. It is asked for on its own, when a book page opens, so
// a change on Hardcover's side can only hide this section, never stop the ratings.
type More struct {
	AudioSeconds int        `json:"audio_seconds"`
	Narrators    []string   `json:"narrators"`
	ASIN         string     `json:"asin"`
	Similar      []BookInfo `json:"-"`
}

type rawMore struct {
	AudioSeconds int             `json:"audio_seconds"`
	SimilarIDs   json.RawMessage `json:"cached_similar_book_ids"`
	Audio        *struct {
		ASIN         string          `json:"asin"`
		AudioSeconds int             `json:"audio_seconds"`
		Contributors json.RawMessage `json:"cached_contributors"`
	} `json:"default_audio_edition"`
}

// MoreAbout looks up the audiobook and up to limit similar books for the Hardcover book id.
func (c *Client) MoreAbout(ctx context.Context, id, limit int) (More, error) {
	var d struct {
		Books []rawMore `json:"books"`
	}
	q := fmt.Sprintf(`query { books(where: {id: {_eq: %d}}, limit: 1) { audio_seconds cached_similar_book_ids
  default_audio_edition { asin audio_seconds cached_contributors } } }`, id)
	if err := c.do(ctx, q, &d); err != nil || len(d.Books) == 0 {
		return More{}, err
	}
	r := d.Books[0]
	m := More{AudioSeconds: r.AudioSeconds}
	if a := r.Audio; a != nil {
		m.ASIN = strings.TrimSpace(a.ASIN)
		if a.AudioSeconds > 0 {
			m.AudioSeconds = a.AudioSeconds
		}
		m.Narrators = narrators(a.Contributors)
	}
	ids := similarIDs(r.SimilarIDs, limit)
	if len(ids) == 0 {
		return m, nil
	}
	var s struct {
		Books []rawBook `json:"books"`
	}
	q = fmt.Sprintf(`query { books(where: {id: {_in: [%s]}}) { %s } }`, strings.Join(ids, ", "), bookFields)
	if err := c.do(ctx, q, &s); err != nil {
		return m, nil // the audiobook is still worth showing
	}
	order := map[int]int{}
	for i, x := range ids {
		n, _ := strconv.Atoi(x)
		order[n] = i
	}
	m.Similar = make([]BookInfo, 0, len(s.Books))
	for _, b := range s.Books {
		m.Similar = append(m.Similar, b.info())
	}
	sortBy(m.Similar, func(b BookInfo) int { return order[b.ID] }) // Hardcover's order: most similar first
	return m, nil
}

// similarIDs reads Hardcover's similar-book list, which is a list of ids (numbers, or objects with an id).
func similarIDs(raw json.RawMessage, limit int) []string {
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		return nil
	}
	var out []string
	for _, x := range list {
		var n int
		if json.Unmarshal(x, &n) != nil {
			var o struct {
				ID int `json:"id"`
			}
			if json.Unmarshal(x, &o) != nil {
				continue
			}
			n = o.ID
		}
		if n > 0 && len(out) < limit {
			out = append(out, strconv.Itoa(n))
		}
	}
	return out
}

// narrators picks the narrators from an edition's contributors ([{author: {name}, contribution: "Narrator"}]).
func narrators(raw json.RawMessage) []string {
	var list []struct {
		Author struct {
			Name string `json:"name"`
		} `json:"author"`
		Contribution string `json:"contribution"`
	}
	_ = json.Unmarshal(raw, &list)
	var out []string
	for _, c := range list {
		if n := strings.TrimSpace(c.Author.Name); n != "" && strings.EqualFold(strings.TrimSpace(c.Contribution), "narrator") && len(out) < 4 {
			out = append(out, n)
		}
	}
	return out
}

func sortBy(b []BookInfo, key func(BookInfo) int) {
	for i := 1; i < len(b); i++ {
		for j := i; j > 0 && key(b[j]) < key(b[j-1]); j-- {
			b[j], b[j-1] = b[j-1], b[j]
		}
	}
}

// InfoByASIN looks up to 25 Audible ASINs in one call and returns each one's book, keyed by the ASIN.
func (c *Client) InfoByASIN(ctx context.Context, asins []string) (map[string]BookInfo, error) {
	var clean []string
	for _, a := range asins {
		a = strings.ToUpper(strings.TrimSpace(a))
		if len(a) == 10 && strings.Trim(a, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") == "" && len(clean) < 25 {
			clean = append(clean, `"`+a+`"`)
		}
	}
	out := map[string]BookInfo{}
	if len(clean) == 0 {
		return out, nil
	}
	var d struct {
		Editions []struct {
			ASIN string  `json:"asin"`
			Book rawBook `json:"book"`
		} `json:"editions"`
	}
	q := fmt.Sprintf(`query { editions(where: {asin: {_in: [%s]}}, limit: 50) { asin book { %s } } }`, strings.Join(clean, ", "), bookFields)
	if err := c.do(ctx, q, &d); err != nil {
		return nil, err
	}
	for _, e := range d.Editions {
		if k := strings.ToUpper(strings.TrimSpace(e.ASIN)); k != "" {
			out[k] = e.Book.info()
		}
	}
	return out, nil
}
