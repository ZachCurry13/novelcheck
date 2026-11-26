// Package freebooks finds books that are free to download legally: Project Gutenberg's public-domain books, through Gutendex (a
// JSON search over its catalog). A book is only offered when its title and author agree with what was asked for.
package freebooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	defaultBase = "https://gutendex.com"
	maxBytes    = 60 << 20 // an e-book from Gutenberg is a few MB; more than this is not one
	cacheFor    = 6 * time.Hour
)

// Match is one free book.
type Match struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
	EPUB   string `json:"-"` // where the file is
}

// Client asks Gutendex (Base is changed in tests) and keeps answers for a few hours, so a list of books is not many requests.
type Client struct {
	Base string
	HTTP *http.Client

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	m  Match
	ok bool
	at time.Time
}

func New() *Client {
	return &Client{Base: defaultBase, HTTP: &http.Client{Timeout: 25 * time.Second}, cache: map[string]cached{}}
}

// squash keeps the letters and digits of s, lowercase.
func squash(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}

// surname is the last name of "First Last" or "Last, First".
func surname(a string) string {
	a = strings.TrimSpace(a)
	if i := strings.Index(a, ","); i > 0 {
		return squash(a[:i])
	}
	f := strings.Fields(a)
	if len(f) == 0 {
		return ""
	}
	return squash(f[len(f)-1])
}

// titleAgrees: the catalog's title is the asked one, or the asked one followed by a subtitle ("Frankenstein; Or, The Modern Prometheus").
func titleAgrees(catalog, asked string) bool {
	c, a := squash(catalog), squash(asked)
	return len(a) >= 3 && (c == a || strings.HasPrefix(c, a))
}

type result struct {
	ID      int `json:"id"`
	Title   string
	Authors []struct {
		Name string `json:"name"`
	} `json:"authors"`
	Languages []string          `json:"languages"`
	Formats   map[string]string `json:"formats"`
}

// Find is the free copy of a book, when there is one: the catalog's title agrees with the title, the author's last name is among
// the catalog's authors (when an author is given), and an EPUB exists.
func (c *Client) Find(ctx context.Context, title, author string) (Match, bool) {
	key := squash(title) + "|" + surname(author)
	if len(squash(title)) < 3 {
		return Match{}, false
	}
	c.mu.Lock()
	if v, ok := c.cache[key]; ok && time.Since(v.at) < cacheFor {
		c.mu.Unlock()
		return v.m, v.ok
	}
	c.mu.Unlock()
	m, ok, err := c.search(ctx, title, author)
	if err == nil { // an error (the service down) is not remembered as "no free copy"
		c.mu.Lock()
		c.cache[key] = cached{m, ok, time.Now()}
		c.mu.Unlock()
	}
	return m, ok
}

func (c *Client) search(ctx context.Context, title, author string) (Match, bool, error) {
	q := strings.TrimSpace(title + " " + author)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Base+"/books/?languages=en&search="+url.QueryEscape(q), nil)
	if err != nil {
		return Match{}, false, err
	}
	req.Header.Set("User-Agent", "NovelCheck")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return Match{}, false, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Match{}, false, fmt.Errorf("gutendex answered %d", res.StatusCode)
	}
	var out struct {
		Results []result `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&out); err != nil {
		return Match{}, false, err
	}
	want := surname(author)
	for _, r := range out.Results {
		epub := r.Formats["application/epub+zip"]
		if epub == "" || !titleAgrees(r.Title, title) {
			continue
		}
		names := make([]string, 0, len(r.Authors))
		hit := want == ""
		for _, a := range r.Authors {
			names = append(names, a.Name)
			hit = hit || surname(a.Name) == want
		}
		if hit {
			return Match{ID: r.ID, Title: r.Title, Author: strings.Join(names, " & "), EPUB: epub}, true, nil
		}
	}
	return Match{}, false, nil
}

// Fetch downloads the book's EPUB, refusing anything that is not an EPUB (a zip) or is too big.
func (c *Client) Fetch(ctx context.Context, m Match) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.EPUB, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "NovelCheck")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the free library answered %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBytes {
		return nil, errors.New("that file is too large to be an e-book")
	}
	if len(data) < 4 || string(data[:2]) != "PK" {
		return nil, errors.New("what came back is not an e-book")
	}
	return data, nil
}
