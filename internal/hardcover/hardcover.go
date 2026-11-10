// Package hardcover reads a person's Hardcover shelves (free API,
// https://api.hardcover.app/v1/graphql, a personal token from
// hardcover.app → Settings → Hardcover API): what they're reading, what
// they've read, and how far along. Limits are about 60 requests a minute;
// one sync is a single request.
package hardcover

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Endpoint is Hardcover's GraphQL address.
const Endpoint = "https://api.hardcover.app/v1/graphql"

// Statuses (Hardcover's status_id).
const (
	WantToRead = 1
	Reading    = 2
	Read       = 3
	Paused     = 4
	DNF        = 5
)

// MinGap is the least time between two calls with one token: Hardcover allows 60 requests a minute, and a
// book sent to it takes several calls. Tests set it to 0.
var MinGap = 1100 * time.Millisecond

// Client talks to Hardcover with one person's token.
type Client struct {
	Token string
	URL   string // "" = Endpoint (tests point it elsewhere)
	HTTP  *http.Client

	mu   sync.Mutex
	last time.Time
}

// pause waits until MinGap has passed since this client's last call.
func (c *Client) pause(ctx context.Context) error {
	c.mu.Lock()
	wait := MinGap - time.Since(c.last)
	c.last = time.Now().Add(max(wait, 0))
	c.mu.Unlock()
	if wait <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
		return nil
	}
}

var errTooMany = errors.New("Hardcover says too many requests; it will try again later")

// CleanToken drops a pasted "Bearer " and stray spaces.
func CleanToken(t string) string {
	t = strings.TrimSpace(t)
	t = strings.TrimPrefix(t, "Bearer ")
	t = strings.TrimPrefix(t, "bearer ")
	return strings.TrimSpace(t)
}

// do makes one call, kept under Hardcover's limit; if Hardcover still says "too many", it waits and tries once more.
func (c *Client) do(ctx context.Context, query string, out any) error {
	if err := c.pause(ctx); err != nil {
		return err
	}
	err := c.doOnce(ctx, query, out)
	if errors.Is(err, errTooMany) && MinGap > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(15 * time.Second):
		}
		return c.doOnce(ctx, query, out)
	}
	return err
}

func (c *Client) doOnce(ctx context.Context, query string, out any) error {
	body, _ := json.Marshal(map[string]string{"query": query})
	url := c.URL
	if url == "" {
		url = Endpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+CleanToken(c.Token))
	req.Header.Set("User-Agent", "NovelCheck (self-hosted reading tracker)") // Hardcover asks scripts to say what they are
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("can't reach Hardcover: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return errors.New("Hardcover didn't accept the token: copy it again from hardcover.app → Settings → Hardcover API (tokens can expire)")
	case http.StatusTooManyRequests:
		return errTooMany
	}
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("unexpected answer from Hardcover (%s)", resp.Status)
	}
	if len(env.Errors) > 0 {
		return fmt.Errorf("Hardcover: %s", env.Errors[0].Message)
	}
	if env.Error != "" {
		return fmt.Errorf("Hardcover: %s", env.Error)
	}
	return json.Unmarshal(env.Data, out)
}

// me unwraps the `me` field, which comes as a list or an object.
func me[T any](raw json.RawMessage) (T, error) {
	var zero T
	raw = bytes.TrimSpace(raw)
	if len(raw) > 0 && raw[0] == '[' {
		var list []T
		if err := json.Unmarshal(raw, &list); err != nil || len(list) == 0 {
			return zero, errors.New("Hardcover returned no account")
		}
		return list[0], nil
	}
	var one T
	err := json.Unmarshal(raw, &one)
	return one, err
}

// Username checks the token and says whose it is.
func (c *Client) Username(ctx context.Context) (string, error) {
	var d struct {
		Me json.RawMessage `json:"me"`
	}
	if err := c.do(ctx, `query { me { id username } }`, &d); err != nil {
		return "", err
	}
	u, err := me[struct {
		Username string `json:"username"`
	}](d.Me)
	return u.Username, err
}

// Book is one book on a person's shelves.
type Book struct {
	Title      string
	Authors    string // the first author
	ISBNs      []string
	Status     int
	Pages      int     // the edition's pages (0 = unknown)
	Progress   int     // pages read (0 = unknown)
	Percent    float64 // 0..1 when known
	Rating     float64 // their stars on Hardcover (0 = none)
	FinishedAt string  // "2026-03-09" when they finished it ("" = unknown)
	UpdatedAt  time.Time
}

const shelvesQuery = `query { me { user_books(where: {status_id: {_in: [1, 2, 3, 4]}}, order_by: {updated_at: desc}, limit: 300) {
  status_id updated_at rating
  book { title pages contributions(limit: 1) { author { name } } }
  edition { pages isbn_10 isbn_13 }
  user_book_reads(order_by: {started_at: desc}, limit: 1) { progress_pages finished_at }
} } }`

type userBook struct {
	StatusID  int      `json:"status_id"`
	UpdatedAt string   `json:"updated_at"`
	Rating    *float64 `json:"rating"`
	Book      struct {
		Title         string `json:"title"`
		Pages         int    `json:"pages"`
		Contributions []struct {
			Author struct {
				Name string `json:"name"`
			} `json:"author"`
		} `json:"contributions"`
	} `json:"book"`
	Edition *struct {
		Pages  int    `json:"pages"`
		ISBN10 string `json:"isbn_10"`
		ISBN13 string `json:"isbn_13"`
	} `json:"edition"`
	Reads []struct {
		ProgressPages int     `json:"progress_pages"`
		FinishedAt    *string `json:"finished_at"`
	} `json:"user_book_reads"`
}

// Shelves returns the books being read and read (newest first, up to 300).
func (c *Client) Shelves(ctx context.Context) ([]Book, error) {
	var d struct {
		Me json.RawMessage `json:"me"`
	}
	if err := c.do(ctx, shelvesQuery, &d); err != nil {
		return nil, err
	}
	u, err := me[struct {
		UserBooks []userBook `json:"user_books"`
	}](d.Me)
	if err != nil {
		return nil, err
	}
	out := make([]Book, 0, len(u.UserBooks))
	for _, ub := range u.UserBooks {
		b := Book{Title: strings.TrimSpace(ub.Book.Title), Status: ub.StatusID, Pages: ub.Book.Pages}
		if b.Title == "" {
			continue
		}
		if len(ub.Book.Contributions) > 0 {
			b.Authors = ub.Book.Contributions[0].Author.Name
		}
		if e := ub.Edition; e != nil {
			if e.Pages > 0 {
				b.Pages = e.Pages
			}
			for _, i := range []string{e.ISBN13, e.ISBN10} {
				if i != "" {
					b.ISBNs = append(b.ISBNs, i)
				}
			}
		}
		if len(ub.Reads) > 0 {
			b.Progress = ub.Reads[0].ProgressPages
			if f := ub.Reads[0].FinishedAt; f != nil && len(*f) >= 10 {
				b.FinishedAt = (*f)[:10] // the day, as Hardcover has it
			}
		}
		switch {
		case b.Status == Read:
			b.Percent = 1
		case b.Pages > 0 && b.Progress > 0:
			b.Percent = min(1, float64(b.Progress)/float64(b.Pages))
		}
		if ub.Rating != nil {
			b.Rating = *ub.Rating
		}
		b.UpdatedAt, _ = time.Parse(time.RFC3339, ub.UpdatedAt)
		out = append(out, b)
	}
	return out, nil
}
