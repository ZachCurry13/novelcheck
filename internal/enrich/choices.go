package enrich

import (
	"context"
	"net/url"
	"strings"
	"unicode"
)

// Choice is one possible match for a typed title, when it isn't clear which
// book was meant.
type Choice struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	ISBN   string `json:"isbn"`
	Year   int    `json:"year"`
}

// Found is the choice as a found book.
func (c Choice) Found() Found { return Found{Title: c.Title, Author: c.Author, ISBN: c.ISBN} }

// Search returns up to n Open Library matches for a typed title and author.
// "Inkheart by Cornelia Funke" searches the part after " by " as the author.
func (c *Client) Search(ctx context.Context, query string, n int) []Choice {
	query = strings.TrimSpace(query)
	var out []Choice
	if title, author, ok := strings.Cut(strings.ToLower(query), " by "); ok && strings.TrimSpace(title) != "" {
		out = c.choices(ctx, url.Values{"title": {strings.TrimSpace(title)}, "author": {strings.TrimSpace(author)}}, n)
	}
	if len(out) == 0 {
		out = c.choices(ctx, url.Values{"q": {query}}, n)
	}
	return out
}

func (c *Client) choices(ctx context.Context, q url.Values, n int) []Choice {
	q.Set("limit", "10")
	q.Set("fields", "title,author_name,isbn,first_publish_year")
	var res struct {
		Docs []struct {
			Title  string   `json:"title"`
			Author []string `json:"author_name"`
			ISBN   []string `json:"isbn"`
			Year   int      `json:"first_publish_year"`
		} `json:"docs"`
	}
	if err := c.getJSON(ctx, c.OpenLibraryURL+"/search.json?"+q.Encode(), &res); err != nil {
		return nil
	}
	var out []Choice
	seen := map[string]bool{}
	for _, d := range res.Docs {
		ch := Choice{Title: d.Title, Year: d.Year}
		if len(d.Author) > 0 {
			ch.Author = d.Author[0]
		}
		for _, isbn := range d.ISBN { // prefer a 13-digit ISBN
			if len(isbn) == 13 || ch.ISBN == "" {
				ch.ISBN = isbn
			}
			if len(ch.ISBN) == 13 {
				break
			}
		}
		key := squash(ch.Title) + "|" + squash(ch.Author)
		if ch.Title == "" || seen[key] || len(out) >= n {
			continue
		}
		seen[key] = true
		out = append(out, ch)
	}
	return out
}

// stopWords don't have to match ("the hobbit by tolkien").
var stopWords = map[string]bool{"the": true, "a": true, "an": true, "by": true, "of": true, "and": true, "to": true,
	"in": true, "on": true, "for": true, "la": true, "le": true, "el": true, "de": true, "der": true, "die": true, "das": true}

// words are the lowercase words of s that must match.
func words(s string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len([]rune(w)) > 1 && !stopWords[w] {
			out = append(out, w)
		}
	}
	return out
}

// squash is s in lowercase letters and digits only ("Ink World" → "inkworld").
func squash(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// hasAll reports whether every word typed is in the choice's title or author.
func hasAll(ws []string, ch Choice) bool {
	text := squash(ch.Title + " " + ch.Author)
	for _, w := range ws {
		if !strings.Contains(text, w) {
			return false
		}
	}
	return true
}

// Sure reports whether the first choice is certainly the book meant: it
// contains every word typed and no other choice does.
func Sure(query string, choices []Choice) bool {
	ws := words(query)
	if len(choices) == 0 || len(ws) == 0 || !hasAll(ws, choices[0]) {
		return false
	}
	for _, ch := range choices[1:] {
		if hasAll(ws, ch) {
			return false
		}
	}
	return true
}
