// Package enrich looks up book blurbs from Open Library and Google Books so
// the LLM classifies from real publisher text instead of guessing — this
// matters most for indie and self-published titles.
package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MinBlurbLen is the shortest description considered useful for analysis.
const MinBlurbLen = 80

type Client struct {
	HTTP           *http.Client
	GoogleAPIKey   string
	OpenLibraryURL string
	GoogleBooksURL string
}

func New(googleKey string) *Client {
	return &Client{
		HTTP:           &http.Client{Timeout: 15 * time.Second},
		GoogleAPIKey:   googleKey,
		OpenLibraryURL: "https://openlibrary.org",
		GoogleBooksURL: "https://www.googleapis.com/books/v1",
	}
}

// Blurb returns the best available summary: Open Library first, then Google
// Books, then the existing (e.g. Calibre) description. source names where it
// came from.
func (c *Client) Blurb(ctx context.Context, title, author, isbn, existing string) (blurb, source string) {
	if b, err := c.openLibrary(ctx, title, author, isbn); err == nil && len(b) >= MinBlurbLen {
		return b, "openlibrary"
	}
	if b, err := c.googleBooks(ctx, title, author, isbn); err == nil && len(b) >= MinBlurbLen {
		return b, "googlebooks"
	}
	return existing, "local"
}

func (c *Client) getJSON(ctx context.Context, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "NovelCheck/1.0 (self-hosted library analyzer)")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	return json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 2<<20)).Decode(out)
}

func (c *Client) openLibrary(ctx context.Context, title, author, isbn string) (string, error) {
	q := url.Values{"limit": {"1"}, "fields": {"key,title"}}
	if isbn != "" {
		q.Set("isbn", isbn)
	} else {
		q.Set("title", title)
		if author != "" {
			q.Set("author", author)
		}
	}
	var search struct {
		Docs []struct {
			Key string `json:"key"`
		} `json:"docs"`
	}
	if err := c.getJSON(ctx, c.OpenLibraryURL+"/search.json?"+q.Encode(), &search); err != nil {
		return "", err
	}
	if len(search.Docs) == 0 || !strings.HasPrefix(search.Docs[0].Key, "/works/") {
		return "", fmt.Errorf("no open library match")
	}
	var work struct {
		Description json.RawMessage `json:"description"`
	}
	if err := c.getJSON(ctx, c.OpenLibraryURL+search.Docs[0].Key+".json", &work); err != nil {
		return "", err
	}
	return olText(work.Description), nil
}

// olText handles Open Library's description being a string or {"value": ...}.
func olText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var obj struct {
		Value string `json:"value"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return strings.TrimSpace(obj.Value)
	}
	return ""
}

func (c *Client) googleBooks(ctx context.Context, title, author, isbn string) (string, error) {
	query := "intitle:" + title
	if isbn != "" {
		query = "isbn:" + isbn
	} else if author != "" {
		query += " inauthor:" + author
	}
	q := url.Values{"q": {query}, "maxResults": {"1"}, "printType": {"books"}}
	if c.GoogleAPIKey != "" {
		q.Set("key", c.GoogleAPIKey)
	}
	var res struct {
		Items []struct {
			VolumeInfo struct {
				Description string `json:"description"`
			} `json:"volumeInfo"`
		} `json:"items"`
	}
	if err := c.getJSON(ctx, c.GoogleBooksURL+"/volumes?"+q.Encode(), &res); err != nil {
		return "", err
	}
	if len(res.Items) == 0 {
		return "", fmt.Errorf("no google books match")
	}
	return strings.TrimSpace(res.Items[0].VolumeInfo.Description), nil
}
