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

	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/titles"
)

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

// missMark is what a miss file holds. Before v1.94 a refused search was written as a miss too, and before v1.99.4 a
// list title like "Neuromancer - William Gibson - 1984" was searched whole; those older files are ignored, so the
// books get searched again. v1.106.0 added sources (Amazon by ISBN-10, the ISBN of the matching book, Hardcover by ISBN,
// main author of "A with B", titles without brackets), so earlier misses are searched again too.
const missMark = "4"

// SearchBook finds a cover for a book that has no ISBN to look up (or whose ISBN has no cover): Open Library's
// search by title and author, then Google Books. A hit is kept as name.jpg. Only a result whose title really is the
// book's title (and whose author matches, when known) is used, so a wrong cover is not put on a book. A real miss is remembered for a week, so the same
// book is not searched again and again. Google Books is asked for the first two spellings of the title only (its
// daily limit is small); Open Library for all of them.
func (c Cache) SearchBook(ctx context.Context, name, title, author string) SearchResult {
	var r SearchResult
	if c.Dir == "" || !storedName.MatchString(name) || strings.TrimSpace(title) == "" {
		return r
	}
	title, author = SplitListTitle(title, author)
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
			if enrich.GoogleResting() {
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
			if strings.Contains(u, "-L.jpg") { // Open Library sometimes has only the medium picture
				if p, info, ok := c.fetch(ctx, strings.Replace(u, "-L.jpg", "-M.jpg", 1), out, miss); ok {
					r.Path, r.Info, r.OK = p, info, true
					return r
				}
			}
		}
	}
	if r.ISBN != "" { // the matching book was found but had no picture: its ISBN may (Open Library, then Amazon)
		if c.ByISBN(ctx, r.ISBN, out, miss) {
			if info, ok := c.have(out); ok {
				r.Path, r.Info, r.OK = out, info, true
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

// listTitle is a title copied from a list: "Neuromancer - William Gibson - 1984", the year last.
var listTitle = regexp.MustCompile(`^(.+?)\s+[-–—]\s+(.+?)\s+[-–—]\s+(1[4-9]\d\d|20\d\d)$`)

// SplitListTitle takes the title and the author out of a list title ("Neuromancer - William Gibson - 1984" with
// the list's name as its author): the book can then be found. Any other title comes back as it was.
func SplitListTitle(title, author string) (string, string) {
	if m := listTitle.FindStringSubmatch(strings.TrimSpace(title)); m != nil {
		return strings.TrimSpace(m[1]), strings.TrimSpace(m[2])
	}
	return title, author
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
	for { // "Title (Full-Cast Edition) (Unabridged)" and "Title [Other Title]": the bare title, and what was in the brackets
		cut := false
		if i := strings.LastIndex(t, " ["); i > 0 && strings.HasSuffix(t, "]") {
			add(t[i+2 : len(t)-1])
			t, cut = strings.TrimSpace(t[:i]), true
		} else if i := strings.LastIndex(t, " ("); i > 0 && strings.HasSuffix(t, ")") {
			t, cut = strings.TrimSpace(t[:i]), true
		}
		if !cut {
			break
		}
		add(t)
	}
	if m := codePrefix.FindStringSubmatch(t); m != nil {
		add(m[1])
		t = m[1]
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

// codePrefix is a short series code and number in front of a title ("HP-1 Harry Potter and the…").
var codePrefix = regexp.MustCompile(`^[A-Za-z]{1,4}-?\d{1,2}\s+(.{4,})$`)

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

// authorNames are the people in an author line ("Rob Knight with Brendan Buhler", "Written By J. K. Rowling,
// Narrated By Jim Dale", "Murray, Brian H. (Writer on real estate)"): the whole line and each part of it, so a book
// matches when any one of them is its author.
func authorNames(a string) []string {
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		for _, p := range []string{"Written By ", "Narrated By ", "By "} {
			s = strings.TrimPrefix(s, p)
		}
		if i := strings.Index(s, " ("); i > 0 {
			s = s[:i]
		}
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	parts := strings.FieldsFunc(a, func(r rune) bool { return r == ';' || r == '&' })
	for _, p := range parts {
		for _, q := range strings.Split(p, " with ") {
			for _, w := range strings.Split(q, " and ") {
				add(w)
			}
		}
	}
	for _, p := range strings.Split(a, ",") { // "Stephen King, Gregorio Vlastelica": a list; "Meyer, Marissa": handled by lastName
		add(p)
	}
	add(a)
	return out
}

// mainAuthor is the author to search for: the first person of an author line.
func mainAuthor(a string) string {
	a = strings.TrimSpace(a)
	for _, p := range []string{"Written By ", "Narrated By ", "By "} {
		a = strings.TrimPrefix(a, p)
	}
	for _, sep := range []string{" with ", " & ", ";", " and "} {
		if i := strings.Index(a, sep); i > 0 {
			a = a[:i]
		}
	}
	if i := strings.Index(a, " ("); i > 0 {
		a = a[:i]
	}
	if i := strings.Index(a, ", "); i > 0 && strings.Contains(a[:i], " ") { // "Stephen King, Gregorio…", not "Meyer, Marissa"
		a = a[:i]
	}
	return strings.TrimSpace(a)
}

func authorOK(found []string, ours string) bool {
	if lastName(ours) == "" || len(found) == 0 {
		return true
	}
	for _, name := range authorNames(ours) {
		want := lastName(name)
		if want == "" {
			continue
		}
		for _, f := range found {
			if strings.Contains(norm(f), want) {
				return true
			}
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
		enrich.RestGoogle()
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
