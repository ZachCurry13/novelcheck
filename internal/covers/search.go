package covers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/zachcurry13/novelcheck/internal/titles"
)

// Search finds a cover for a book that has no ISBN to look up (or whose ISBN has no cover): Open
// Library's search by title and author, then Google Books. A hit is kept as name.jpg. Only a result whose title
// really is the book's title (and whose author matches, when known) is used, so a wrong cover is not put on a book.
func (c Cache) Search(ctx context.Context, name, title, author string) (string, os.FileInfo, bool) {
	r := c.SearchBook(ctx, name, title, author)
	return r.Path, r.Info, r.OK
}

// SearchResult is what SearchBook found: the cover (when OK), an ISBN the matching book carried, and whether a
// place refused to answer (Google's daily limit, the network). After a refusal "nothing found" is not the last
// word, so it is not remembered as a miss.
type SearchResult struct {
	Path    string
	Info    os.FileInfo
	OK      bool
	ISBN    string
	Refused bool
}

// missMark is what a miss file holds. Before v1.94 a refused search was written as a miss too; those old files
// (empty) are ignored, so the books get searched again.
const missMark = "2"

// SearchBook is Search with the ISBN found and the refusals. A real miss is remembered for a week, so the same
// book is not searched again and again. Google Books is asked for the first two spellings of the title only (its
// daily limit is small); Open Library for all of them.
func (c Cache) SearchBook(ctx context.Context, name, title, author string) SearchResult {
	var r SearchResult
	if c.Dir == "" || !storedName.MatchString(name) || strings.TrimSpace(title) == "" {
		return r
	}
	out := filepath.Join(c.Dir, name+".jpg")
	if info, ok := c.have(out); ok {
		return SearchResult{Path: out, Info: info, OK: true}
	}
	miss := out + ".miss"
	if c.Missed(name) {
		return r
	}
	_ = os.Remove(out + ".none")
	for i, t := range searchTitles(title) {
		finds := []func(context.Context, string, string, *bool) (string, string){c.openLibraryCoverURL}
		if !c.NoGoogle && i < 2 {
			if c.GoogleResting() {
				r.Refused = true
			} else {
				finds = append(finds, c.googleCoverURL)
			}
		}
		for _, find := range finds {
			u, isbn := find(ctx, t, author, &r.Refused)
			if r.ISBN == "" {
				r.ISBN = isbn
			}
			if u == "" {
				continue
			}
			if p, info, ok := c.fetch(ctx, u, out, miss); ok {
				r.Path, r.Info, r.OK = p, info, true
				return r
			}
		}
	}
	if !r.Refused {
		_ = os.MkdirAll(c.Dir, 0o755)
		_ = os.WriteFile(miss, []byte(missMark), 0o644)
	}
	return r
}

// Missed says whether this book was searched in the last week and nothing was found (Search then skips it).
func (c Cache) Missed(name string) bool {
	if c.Dir == "" {
		return false
	}
	p := filepath.Join(c.Dir, name+".jpg.miss")
	info, err := os.Stat(p)
	if err != nil || time.Since(info.ModTime()) >= 7*24*time.Hour {
		return false
	}
	b, _ := os.ReadFile(p)
	return string(b) == missMark
}

// Forget drops the memory of a miss, so the next Search looks again.
func (c Cache) Forget(name string) {
	if c.Dir != "" {
		_ = os.Remove(filepath.Join(c.Dir, name+".jpg.miss"))
	}
}

// searchTitles is what to search for a library title, best first: the title tidied of track and series
// numbers ("01 - Guards! Guards!"), without a series in brackets at the end ("Stars Above (The Lunar
// Chronicles)"), the part after a numbered series prefix ("Percy Jackson 2 - The Sea of Monsters"), and the
// part before a subtitle. Each is searched only once.
func searchTitles(raw string) []string {
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		for _, o := range out {
			if strings.EqualFold(o, s) {
				return
			}
		}
		if len([]rune(s)) >= 2 {
			out = append(out, s)
		}
	}
	t := titles.Clean(raw)
	add(t)
	if i := strings.LastIndex(t, " ("); i > 0 && strings.HasSuffix(t, ")") {
		t = strings.TrimSpace(t[:i])
		add(t)
	}
	if m := seriesPrefix.FindStringSubmatch(t); m != nil {
		add(m[1])
	}
	for _, s := range out {
		if i := strings.Index(s, ":"); i > 0 {
			add(s[:i])
			break
		}
	}
	return out
}

// seriesPrefix is "Series Name 2 - Title" or "Series #2: Title": the title after the number.
var seriesPrefix = regexp.MustCompile(`^.+?\s#?\d+(?:\.\d+)?\s*[-–—:]\s*(.+)$`)

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

// getJSON asks u and reads its answer into into. A failure that is not a plain "no such thing" (the network, a
// daily limit, a server error) sets *refused, when refused is not nil; Google's limit also rests Google for the day.
func (c Cache) getJSON(ctx context.Context, u string, into any, refused *bool) bool {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		setTrue(refused)
		return false
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	switch {
	case res.StatusCode == http.StatusOK:
		return err == nil && json.Unmarshal(body, into) == nil
	case res.StatusCode == http.StatusNotFound:
		return false
	}
	setTrue(refused)
	if (res.StatusCode == http.StatusTooManyRequests || res.StatusCode == http.StatusForbidden) && strings.Contains(u, "/books/v1/") {
		c.Limits.restGoogle()
	}
	return false
}

func setTrue(b *bool) {
	if b != nil {
		*b = true
	}
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
