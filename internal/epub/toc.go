package epub

import (
	"archive/zip"
	"fmt"
	"html"
	"regexp"
	"strings"
)

// Books reads an omnibus or box set's own table of contents and returns the books inside it, in order: the
// top-level entries that have chapters under them (front and back matter left out). It returns nothing when
// the contents are a flat list of chapters, so the caller can ask elsewhere. EPUB 2's toc.ncx and EPUB 3's
// nav document are both read.
func Books(file string) ([]string, error) {
	zr, err := zip.OpenReader(file)
	if err != nil {
		return nil, fmt.Errorf("not a readable EPUB: %w", err)
	}
	defer zr.Close()
	var ncx, nav *zip.File
	for _, f := range zr.File {
		n := strings.ToLower(f.Name)
		switch {
		case strings.HasSuffix(n, ".ncx"):
			ncx = f
		case strings.HasSuffix(n, "nav.xhtml") || strings.HasSuffix(n, "nav.html") || strings.HasSuffix(n, "toc.xhtml"):
			nav = f
		}
	}
	var top []tocEntry
	if ncx != nil {
		var doc struct {
			Points []navPoint `xml:"navMap>navPoint"`
		}
		if readXML(ncx, &doc) == nil {
			for _, p := range doc.Points {
				top = append(top, p.entry())
			}
		}
	}
	if len(top) == 0 && nav != nil {
		if raw, err := readAll(nav); err == nil {
			top = navEntries(string(raw))
		}
	}
	var out []string
	for _, e := range top {
		if e.children > 0 && !matter.MatchString(e.title) {
			out = append(out, e.title)
		}
	}
	if len(out) < 2 { // one entry with chapters is the whole book, not a set
		return nil, nil
	}
	return out, nil
}

type tocEntry struct {
	title    string
	children int
}

type navPoint struct {
	Label  string     `xml:"navLabel>text"`
	Points []navPoint `xml:"navPoint"`
}

func (p navPoint) entry() tocEntry {
	return tocEntry{title: tidy(p.Label), children: len(p.Points)}
}

// matter is front and back matter that sits at the top of a set's contents beside the books.
var matter = regexp.MustCompile(`(?i)^(cover|title( page)?|copyright|contents|table of contents|dedication|epigraph|acknowledg|about the (author|publisher)|also by|other books|books by|praise|foreword|preface|introduction|afterword|bonus|excerpt|sneak peek|newsletter|map|glossary|cast of characters|front matter|back matter)`)

var (
	liOpen  = regexp.MustCompile(`(?is)<(/?)(ol|li)\b[^>]*>`)
	navTOC  = regexp.MustCompile(`(?is)<nav\b[^>]*toc[^>]*>(.*?)</nav>`)
	anyTag  = regexp.MustCompile(`<[^>]*>`)
	spaces  = regexp.MustCompile(`\s+`)
	leading = regexp.MustCompile(`(?i)^(?:book|volume|vol\.?|part)\s+(?:\d+|[ivx]+|one|two|three|four|five|six|seven|eight|nine|ten)\s*[:.\-–—]\s*`)
)

// navEntries reads the top level of an EPUB 3 nav document's table of contents: each first-level <li>'s own
// text, and how many <li> sit under it.
func navEntries(doc string) []tocEntry {
	if m := navTOC.FindStringSubmatch(doc); m != nil {
		doc = m[1]
	}
	var out []tocEntry
	depth := 0 // how many <ol> deep
	var cur *tocEntry
	var text strings.Builder
	last := 0
	for _, m := range liOpen.FindAllStringSubmatchIndex(doc, -1) {
		chunk := doc[last:m[0]]
		last = m[1]
		closing, tag := doc[m[2]:m[3]] == "/", strings.ToLower(doc[m[4]:m[5]])
		if cur != nil && depth == 1 {
			text.WriteString(chunk)
		}
		switch {
		case tag == "ol" && !closing:
			if depth == 1 && cur != nil && cur.title == "" {
				cur.title = tidy(text.String())
			}
			depth++
		case tag == "ol" && closing:
			depth--
		case tag == "li" && !closing && depth == 1:
			out = append(out, tocEntry{})
			cur = &out[len(out)-1]
			text.Reset()
		case tag == "li" && !closing && depth == 2 && cur != nil:
			cur.children++
		case tag == "li" && closing && depth == 1 && cur != nil && cur.title == "":
			cur.title = tidy(text.String())
		}
	}
	return out
}

// tidy is an entry's plain text, without a "Book 1:" in front.
func tidy(s string) string {
	s = html.UnescapeString(anyTag.ReplaceAllString(s, " "))
	s = strings.TrimSpace(spaces.ReplaceAllString(s, " "))
	return strings.TrimSpace(leading.ReplaceAllString(s, ""))
}

