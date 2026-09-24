package enrich

import (
	"context"
	"net/url"
	"strings"
)

// Found is a book identified from what someone typed or photographed.
type Found struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	ISBN   string `json:"isbn"`
}

// CleanISBN returns an ISBN-10/13 without dashes or spaces ("" if s isn't one).
func CleanISBN(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		switch {
		case r >= '0' && r <= '9', r == 'X':
			b.WriteRune(r)
		case r == '-' || r == ' ':
		default:
			return ""
		}
	}
	isbn := b.String()
	if strings.Contains(isbn[:max(len(isbn)-1, 0)], "X") || (len(isbn) != 10 && len(isbn) != 13) ||
		(len(isbn) == 13 && strings.Contains(isbn, "X")) {
		return ""
	}
	return isbn
}

// Find identifies a book from free text ("fourth wing yarros") or an ISBN.
func (c *Client) Find(ctx context.Context, query string) (Found, bool) {
	q := url.Values{}
	if isbn := CleanISBN(query); isbn != "" {
		q.Set("isbn", isbn)
	} else {
		q.Set("q", strings.TrimSpace(query))
	}
	return c.search(ctx, q)
}

// FindTitle looks up a title and author read off a cover, for the catalogued
// spelling (and an ISBN).
func (c *Client) FindTitle(ctx context.Context, title, author string) (Found, bool) {
	q := url.Values{"title": {title}}
	if author != "" {
		q.Set("author", author)
	}
	return c.search(ctx, q)
}

func (c *Client) search(ctx context.Context, q url.Values) (Found, bool) {
	q.Set("limit", "1")
	q.Set("fields", "title,author_name,isbn")
	var res struct {
		Docs []struct {
			Title  string   `json:"title"`
			Author []string `json:"author_name"`
			ISBN   []string `json:"isbn"`
		} `json:"docs"`
	}
	if err := c.getJSON(ctx, c.OpenLibraryURL+"/search.json?"+q.Encode(), &res); err != nil || len(res.Docs) == 0 || res.Docs[0].Title == "" {
		return Found{}, false
	}
	d := res.Docs[0]
	f := Found{Title: d.Title}
	if len(d.Author) > 0 {
		f.Author = d.Author[0]
	}
	for _, isbn := range d.ISBN { // prefer a 13-digit ISBN
		if len(isbn) == 13 || f.ISBN == "" {
			f.ISBN = isbn
		}
		if len(f.ISBN) == 13 {
			break
		}
	}
	return f, true
}
