package covers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// Search finds a cover for a book that has no ISBN to look up (or whose ISBN has no cover): Open
// Library's search by title and author, then Google Books. A hit is kept as name.jpg; a miss is
// remembered for two weeks so the same book is not searched again and again. Only a result whose title
// really is the book's title (and whose author matches, when known) is used, so a wrong cover is not
// put on a book.
func (c Cache) Search(ctx context.Context, name, title, author string) (string, os.FileInfo, bool) {
	if c.Dir == "" || !storedName.MatchString(name) || strings.TrimSpace(title) == "" {
		return "", nil, false
	}
	out := filepath.Join(c.Dir, name+".jpg")
	if info, err := os.Stat(out); err == nil {
		return out, info, true
	}
	miss := out + ".none"
	if info, err := os.Stat(miss); err == nil && time.Since(info.ModTime()) < 14*24*time.Hour {
		return "", nil, false
	}
	for _, find := range []func(context.Context, string, string) string{c.openLibraryCoverURL, c.googleCoverURL} {
		if u := find(ctx, title, author); u != "" {
			if p, info, ok := c.fetch(ctx, u, out, miss); ok {
				return p, info, true
			}
		}
	}
	_ = os.MkdirAll(c.Dir, 0o755)
	_ = os.WriteFile(miss, nil, 0o644)
	return "", nil, false
}

// norm boils a title or name down to its letters and digits, without a leading "the".
func norm(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	t := b.String()
	return strings.TrimPrefix(t, "the")
}

// sameTitle says whether a found title is ours (before any subtitle, so "Dune" and "Dune: Messiah" differ
// only when the whole titles do not agree up to the colon).
func sameTitle(found, ours string) bool {
	cut := func(t string) string {
		if i := strings.Index(t, ":"); i > 0 {
			return t[:i]
		}
		return t
	}
	a, b := norm(cut(found)), norm(cut(ours))
	return a != "" && a == b
}

func lastName(a string) string {
	f := strings.Fields(strings.ReplaceAll(a, ",", " "))
	if len(f) == 0 {
		return ""
	}
	if strings.Contains(a, ",") { // "Meyer, Marissa"
		return norm(f[0])
	}
	return norm(f[len(f)-1])
}

func authorOK(found []string, ours string) bool {
	want := lastName(ours)
	if want == "" || len(found) == 0 {
		return true
	}
	for _, f := range found {
		if strings.Contains(norm(f), want) {
			return true
		}
	}
	return false
}

func (c Cache) getJSON(ctx context.Context, u string, into any) bool {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	return err == nil && res.StatusCode == http.StatusOK && json.Unmarshal(body, into) == nil
}

func (c Cache) openLibraryCoverURL(ctx context.Context, title, author string) string {
	base := c.SearchURL
	if base == "" {
		base = "https://openlibrary.org"
	}
	q := url.Values{"title": {title}, "limit": {"6"}, "fields": {"title,author_name,cover_i"}}
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
		return ""
	}
	cover := c.OpenLibraryURL
	if cover == "" {
		cover = openLibrary
	}
	for _, d := range res.Docs {
		if d.Cover > 0 && sameTitle(d.Title, title) && authorOK(d.Author, author) {
			return cover + "/b/id/" + url.PathEscape(itoa(d.Cover)) + "-M.jpg?default=false"
		}
	}
	return ""
}

func (c Cache) googleCoverURL(ctx context.Context, title, author string) string {
	base := c.GoogleURL
	if base == "" {
		base = "https://www.googleapis.com"
	}
	q := "intitle:" + title
	if lastName(author) != "" {
		q += " inauthor:" + author
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
	if !c.getJSON(ctx, base+"/books/v1/volumes?"+url.Values{"q": {q}, "maxResults": {"6"}}.Encode(), &res) {
		return ""
	}
	for _, it := range res.Items {
		if it.Info.Images.Thumb != "" && sameTitle(it.Info.Title, title) && authorOK(it.Info.Authors, author) {
			u := strings.Replace(it.Info.Images.Thumb, "http://", "https://", 1)
			return strings.Replace(u, "&edge=curl", "", 1)
		}
	}
	return ""
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
