package covers

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Choosing a cover by hand (Admin → Library cleanup → Review covers): the candidates for a book, from Open Library
// and Google Books under each tidied form of its title, and saving the one a person picks.

// Candidate is a cover a person can pick.
type Candidate struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	URL    string `json:"url"` // the image on Open Library or Google Books
	From   string `json:"from"`
}

// Titles is what Review covers suggests as cleaner titles for a library title (series numbers and brackets left
// out), best first; the first is the one offered as "Use this title" when it differs.
func Titles(raw string) []string { return searchTitles(raw) }

// Candidates lists up to max covers for a book, without the strict title check of the automatic finder: a person
// chooses. query, when given, is searched instead of the book's own title.
func (c Cache) Candidates(ctx context.Context, title, author, query string, max int) []Candidate {
	seen := map[string]bool{}
	var out []Candidate
	add := func(x Candidate) {
		if x.URL != "" && !seen[x.URL] && len(out) < max {
			seen[x.URL] = true
			out = append(out, x)
		}
	}
	tries := searchTitles(title)
	if q := strings.TrimSpace(query); q != "" {
		tries, author = []string{q}, ""
	}
	for _, t := range tries {
		for _, x := range c.openLibraryAll(ctx, t, author) {
			add(x)
		}
		for _, x := range c.googleAll(ctx, t, author) {
			add(x)
		}
		if len(out) >= max {
			break
		}
	}
	return out
}

func (c Cache) openLibraryAll(ctx context.Context, title, author string) []Candidate {
	base := c.SearchURL
	if base == "" {
		base = "https://openlibrary.org"
	}
	q := url.Values{"title": {title}, "limit": {"8"}, "fields": {"title,author_name,cover_i"}}
	if lastName(author) != "" {
		q.Set("author", author)
	}
	var res struct {
		Docs []struct {
			Title  string   `json:"title"`
			Author []string `json:"author_name"`
			Cover  int64    `json:"cover_i"`
		} `json:"docs"`
	}
	if !c.getJSON(ctx, base+"/search.json?"+q.Encode(), &res) {
		return nil
	}
	cover := c.OpenLibraryURL
	if cover == "" {
		cover = openLibrary
	}
	var out []Candidate
	for _, d := range res.Docs {
		if d.Cover > 0 {
			out = append(out, Candidate{Title: d.Title, Author: strings.Join(d.Author, ", "), URL: cover + "/b/id/" + itoa(d.Cover) + "-M.jpg?default=false", From: "Open Library"})
		}
	}
	return out
}

func (c Cache) googleAll(ctx context.Context, title, author string) []Candidate {
	base := c.GoogleURL
	if base == "" {
		base = "https://www.googleapis.com"
	}
	q := "intitle:" + title
	if lastName(author) != "" {
		q += " inauthor:" + author
	}
	params := url.Values{"q": {q}, "maxResults": {"8"}}
	if c.GoogleKey != nil {
		if k := strings.TrimSpace(c.GoogleKey()); k != "" {
			params.Set("key", k)
		}
	}
	var res struct {
		Items []struct {
			Info struct {
				Title   string   `json:"title"`
				Authors []string `json:"authors"`
				Images  struct {
					Thumb string `json:"thumbnail"`
				} `json:"imageLinks"`
			} `json:"volumeInfo"`
		} `json:"items"`
	}
	if !c.getJSON(ctx, base+"/books/v1/volumes?"+params.Encode(), &res) {
		return nil
	}
	var out []Candidate
	for _, it := range res.Items {
		if u := it.Info.Images.Thumb; u != "" {
			u = strings.Replace(strings.Replace(u, "http://", "https://", 1), "&edge=curl", "", 1)
			out = append(out, Candidate{Title: it.Info.Title, Author: strings.Join(it.Info.Authors, ", "), URL: u, From: "Google Books"})
		}
	}
	return out
}

// AllowedImage says whether u is an image address Review covers may show or save: only the two catalogues it
// searches, over https.
func AllowedImage(u string) bool {
	p, err := url.Parse(u)
	if err != nil || p.Scheme != "https" {
		return false
	}
	h := strings.ToLower(p.Hostname())
	return h == "covers.openlibrary.org" || h == "books.google.com" || strings.HasSuffix(h, ".googleusercontent.com")
}

// Choose saves the image at u as the book's cover (name.jpg), replacing any cover found before, and forgets an
// earlier miss.
func (c Cache) Choose(ctx context.Context, name, u string) bool {
	if c.Dir == "" || !storedName.MatchString(name) {
		return false
	}
	out := filepath.Join(c.Dir, name+".jpg")
	_ = os.Remove(out)
	_ = os.Remove(out + ".miss")
	_, _, ok := c.fetch(ctx, u, out, out+".none")
	return ok
}
