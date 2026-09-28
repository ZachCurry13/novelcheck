// Package discover fills the Discover tab: New York Times best-seller lists
// (with a free API key), Open Library classics, and Open Library fallbacks
// when there's no key. Lists refresh daily; their new books are rated in the
// background a few dozen a day.
package discover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// ErrNYTKey means the New York Times refused the API key.
var ErrNYTKey = errors.New("the New York Times refused the API key; check it in Admin → Delivery & Services → Discover")

// Client fetches the lists. The URLs can point at test servers.
type Client struct {
	HTTP           *http.Client
	NYTURL         string
	OpenLibraryURL string
}

func NewClient() *Client {
	return &Client{
		HTTP:           &http.Client{Timeout: 20 * time.Second},
		NYTURL:         "https://api.nytimes.com/svc/books/v3",
		OpenLibraryURL: "https://openlibrary.org",
	}
}

func (c *Client) getJSON(ctx context.Context, u string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "NovelCheck/1.0 (self-hosted library analyzer)")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return res.StatusCode, fmt.Errorf("%s", res.Status)
	}
	return res.StatusCode, json.NewDecoder(http.MaxBytesReader(nil, res.Body, 4<<20)).Decode(out)
}

// NYTList fetches one current best-seller list, e.g. "young-adult-hardcover".
func (c *Client) NYTList(ctx context.Context, key, list string) ([]store.DiscoverEntry, error) {
	var out struct {
		Results struct {
			Books []struct {
				Rank        int    `json:"rank"`
				Title       string `json:"title"`
				Author      string `json:"author"`
				ISBN13      string `json:"primary_isbn13"`
				ISBN10      string `json:"primary_isbn10"`
				Description string `json:"description"`
				Image       string `json:"book_image"`
				Weeks       int    `json:"weeks_on_list"`
			} `json:"books"`
		} `json:"results"`
	}
	u := c.NYTURL + "/lists/current/" + url.PathEscape(list) + ".json?api-key=" + url.QueryEscape(key)
	switch code, err := c.getJSON(ctx, u, &out); {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return nil, ErrNYTKey
	case code == http.StatusTooManyRequests:
		return nil, errors.New("the New York Times asked NovelCheck to slow down; it tries again tomorrow")
	case err != nil:
		return nil, fmt.Errorf("New York Times list %s: %w", list, err)
	}
	var entries []store.DiscoverEntry
	for _, b := range out.Results.Books {
		isbn := b.ISBN13
		if isbn == "" {
			isbn = b.ISBN10
		}
		entries = append(entries, store.DiscoverEntry{Rank: b.Rank, Title: TitleCase(b.Title), Author: strings.TrimSpace(b.Author),
			ISBN: isbn, Description: strings.TrimSpace(b.Description), CoverURL: b.Image, WeeksOnList: b.Weeks})
	}
	return entries, nil
}

// olWork is a book in an Open Library subject or trending list.
type olWork struct {
	Key        string   `json:"key"`
	Title      string   `json:"title"`
	AuthorName []string `json:"author_name"` // trending
	Authors    []struct {
		Name string `json:"name"`
	} `json:"authors"` // subjects
	CoverID  int      `json:"cover_id"` // subjects
	CoverI   int      `json:"cover_i"`  // trending
	Language []string `json:"language"`
}

func (c *Client) olEntry(w olWork) store.DiscoverEntry {
	e := store.DiscoverEntry{Title: strings.TrimSpace(w.Title)}
	if len(w.Authors) > 0 {
		e.Author = w.Authors[0].Name
	} else if len(w.AuthorName) > 0 {
		e.Author = w.AuthorName[0]
	}
	if id := max(w.CoverID, w.CoverI); id > 0 {
		e.CoverURL = "https://covers.openlibrary.org/b/id/" + strconv.Itoa(id) + "-M.jpg"
	}
	if strings.HasPrefix(w.Key, "/works/") {
		e.Link = "https://openlibrary.org" + w.Key
	}
	return e
}

// OLSubject fetches an Open Library subject's best-known books, e.g. "classics".
func (c *Client) OLSubject(ctx context.Context, subject string, limit int) ([]store.DiscoverEntry, error) {
	var out struct {
		Works []olWork `json:"works"`
	}
	if _, err := c.getJSON(ctx, c.OpenLibraryURL+"/subjects/"+url.PathEscape(subject)+".json?limit="+strconv.Itoa(limit), &out); err != nil {
		return nil, fmt.Errorf("Open Library %s: %w", subject, err)
	}
	var entries []store.DiscoverEntry
	for _, w := range out.Works {
		entries = append(entries, c.olEntry(w))
	}
	return entries, nil
}

// OLTrending fetches this week's most-read English books on Open Library
// (the Popular row's fallback without a New York Times key).
func (c *Client) OLTrending(ctx context.Context, limit int) ([]store.DiscoverEntry, error) {
	var out struct {
		Works []olWork `json:"works"`
	}
	if _, err := c.getJSON(ctx, c.OpenLibraryURL+"/trending/weekly.json?limit="+strconv.Itoa(limit*3), &out); err != nil {
		return nil, fmt.Errorf("Open Library trending: %w", err)
	}
	var entries []store.DiscoverEntry
	for _, w := range out.Works {
		if len(entries) < limit && (len(w.Language) == 0 || contains(w.Language, "eng")) {
			entries = append(entries, c.olEntry(w))
		}
	}
	return entries, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
