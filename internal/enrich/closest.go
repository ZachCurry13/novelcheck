package enrich

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Titles that are typed, scanned or copied badly ("Unnatural.Exprosure" for Unnatural Exposure) are found
// among the author's own books on Open Library: the one whose title is closest, allowing a few wrong letters.

var authorBooks sync.Map // author -> authorList, so each author is asked for once a day

type authorList struct {
	at    time.Time
	books []Found
}

// AuthorTitles lists up to 300 of an author's books from Open Library (title and first author).
func (c *Client) AuthorTitles(ctx context.Context, author string) []Found {
	author = strings.TrimSpace(author)
	if author == "" {
		return nil
	}
	if v, ok := authorBooks.Load(strings.ToLower(author)); ok && time.Since(v.(authorList).at) < 24*time.Hour {
		return v.(authorList).books
	}
	q := url.Values{"author": {author}, "limit": {"300"}, "fields": {"title,author_name,isbn"}}
	var res struct {
		Docs []struct {
			Title  string   `json:"title"`
			Author []string `json:"author_name"`
		} `json:"docs"`
	}
	if err := c.getJSON(ctx, c.OpenLibraryURL+"/search.json?"+q.Encode(), &res); err != nil {
		return nil
	}
	var out []Found
	for _, d := range res.Docs {
		if d.Title != "" && len(d.Author) > 0 {
			out = append(out, Found{Title: d.Title, Author: d.Author[0]})
		}
	}
	authorBooks.Store(strings.ToLower(author), authorList{time.Now(), out})
	return out
}

// ClosestTitle picks the author's book a messy title means: dots and underscores read as spaces, case and
// punctuation ignored, and a few wrong letters allowed (one in seven, at least one). Only a clear winner
// counts: two different books equally close mean no answer. A catalog title with a subtitle is compared, and
// given, without it.
func ClosestTitle(title, author string, books []Found) (Found, bool) {
	want := titleKey(title)
	if len([]rune(want)) < 3 {
		return Found{}, false
	}
	best, bestD, tie := Found{}, 1<<30, false
	for _, b := range books {
		if !sameAuthor(b.Author, author) {
			continue
		}
		t := b.Title
		if i := strings.Index(t, ":"); i > 0 && squash(t[:i]) != "" {
			t = strings.TrimSpace(t[:i])
		}
		d := editDistance(want, titleKey(t))
		switch {
		case d < bestD:
			best, bestD, tie = Found{Title: t, Author: b.Author}, d, false
		case d == bestD && titleKey(t) != titleKey(best.Title):
			tie = true
		}
	}
	allowed := max(1, len([]rune(want))/7)
	if tie || bestD > allowed {
		return Found{}, false
	}
	return best, true
}

// titleKey is a title boiled down for comparing: dots and underscores as spaces, "&" as "and", no leading
// "The", "A" or "An", then letters and digits only ("The Last Precinct" and "Last.Precinct" agree).
func titleKey(s string) string {
	s = strings.ToLower(strings.NewReplacer(".", " ", "_", " ", "&", " and ").Replace(s))
	f := strings.Fields(s)
	if len(f) > 1 && (f[0] == "the" || f[0] == "a" || f[0] == "an") {
		f = f[1:]
	}
	return squash(strings.Join(f, " "))
}
