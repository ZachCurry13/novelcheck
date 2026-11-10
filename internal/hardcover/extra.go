package hardcover

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// FindEditionByTitle finds a book that has no ISBN here: Hardcover's own search for its title and author, then
// only a result whose title AND an author match exactly (ignoring case, punctuation and numbering), so a
// look-alike can never be written to.
func (c *Client) FindEditionByTitle(ctx context.Context, title, author string) (Edition, bool, error) {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(author) == "" {
		return Edition{}, false, nil // a title alone is too weak to write to anyone's shelves
	}
	lit, _ := json.Marshal(strings.TrimSpace(title + " " + author)) // a quoted, escaped GraphQL string
	var s struct {
		Search struct {
			IDs []json.RawMessage `json:"ids"`
		} `json:"search"`
	}
	if err := c.do(ctx, `query { search(query: `+string(lit)+`, query_type: "Book", per_page: 5, page: 1) { ids } }`, &s); err != nil {
		return Edition{}, false, err
	}
	var ids []string
	for _, raw := range s.Search.IDs {
		if n, err := strconv.Atoi(strings.Trim(string(raw), `"`)); err == nil {
			ids = append(ids, strconv.Itoa(n))
		}
	}
	if len(ids) == 0 {
		return Edition{}, false, nil
	}
	var d struct {
		Books []struct {
			ID            int    `json:"id"`
			Title         string `json:"title"`
			Contributions []struct {
				Author struct {
					Name string `json:"name"`
				} `json:"author"`
			} `json:"contributions"`
			Editions []struct {
				ID    int `json:"id"`
				Pages int `json:"pages"`
			} `json:"editions"`
		} `json:"books"`
	}
	q := fmt.Sprintf(`query { books(where: {id: {_in: [%s]}}) { id title contributions(limit: 3) { author { name } } editions(order_by: {users_count: desc_nulls_last}, limit: 1) { id pages } } }`, strings.Join(ids, ", "))
	if err := c.do(ctx, q, &d); err != nil {
		return Edition{}, false, err
	}
	want := store.NormKey(title, author)
	for _, b := range d.Books {
		for _, con := range b.Contributions {
			if store.NormKey(b.Title, con.Author.Name) == want && len(b.Editions) > 0 {
				return Edition{ID: b.Editions[0].ID, BookID: b.ID, Pages: b.Editions[0].Pages}, true, nil
			}
		}
	}
	return Edition{}, false, nil
}

// Goal is one of the person's reading goals (Hardcover's reading challenge).
type Goal struct {
	Metric      string  `json:"metric"` // "book" or "page"
	Goal        float64 `json:"goal"`
	Progress    float64 `json:"progress"`
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	Description string  `json:"description,omitempty"`
}

// Goals lists the person's reading goals, newest first.
func (c *Client) Goals(ctx context.Context) ([]Goal, error) {
	var d struct {
		Me json.RawMessage `json:"me"`
	}
	if err := c.do(ctx, `query { me { goals(order_by: {end_date: desc}, limit: 10) { goal metric progress start_date end_date description } } }`, &d); err != nil {
		return nil, err
	}
	u, err := me[struct {
		Goals []Goal `json:"goals"`
	}](d.Me)
	return u.Goals, err
}

// PushShelf puts a book on the person's Hardcover shelves as Want to read (1) or Paused (4), and never moves
// a book that is already further along: a Paused only replaces Reading.
func (c *Client) PushShelf(ctx context.Context, ed Edition, status int) error {
	ub, err := c.FindUserBook(ctx, ed.BookID)
	if err != nil {
		return err
	}
	switch {
	case ub.ID == 0:
		return c.mutate(ctx, fmt.Sprintf(`mutation { insert_user_book(object: {book_id: %d, edition_id: %d, status_id: %d}) { id error } }`, ed.BookID, ed.ID, status))
	case status == Paused && ub.Status == Reading:
		return c.mutate(ctx, fmt.Sprintf(`mutation { update_user_book(id: %d, object: {status_id: %d}) { id error } }`, ub.ID, status))
	}
	return nil
}
