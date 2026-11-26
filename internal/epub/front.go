package epub

import (
	"archive/zip"
	"errors"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// Front finds where a book says what it is: the package file's metadata (title, author, publisher, identifiers), the title
// page, and the copyright page with its ISBN. Books mark those pages three ways: the package's <guide> ("title-page",
// "copyright-page"), the EPUB 3 navigation file's landmarks (epub:type="titlepage"), and, failing both, their file names
// ("titlepage.xhtml", "copyright.xhtml") or, for the copyright page, its words ("All rights reserved", ©, ISBN).
// Only the first pages of the book are opened.

// FrontMatter is what was found; any part may be empty.
type FrontMatter struct {
	Title, Author, Publisher string
	ISBNs                    []string // valid ISBNs from the metadata and the copyright page, as digits
	TitlePage                string
	CopyrightPage            string
	Opening                  string // the first words of the text itself, for books with neither page
}

const (
	frontSections = 14  // how far into the book to look for the title and copyright pages
	pageWords     = 350 // how much of each page is kept
)

type opfFull struct {
	Metadata struct {
		Titles      []string `xml:"title"`
		Creators    []string `xml:"creator"`
		Publishers  []string `xml:"publisher"`
		Identifiers []string `xml:"identifier"`
	} `xml:"metadata"`
	Manifest []struct {
		ID    string `xml:"id,attr"`
		Href  string `xml:"href,attr"`
		Type  string `xml:"media-type,attr"`
		Props string `xml:"properties,attr"`
	} `xml:"manifest>item"`
	Spine []struct {
		IDRef string `xml:"idref,attr"`
	} `xml:"spine>itemref"`
	Guide []struct {
		Type string `xml:"type,attr"`
		Href string `xml:"href,attr"`
	} `xml:"guide>reference"`
}

var (
	landmarkA = regexp.MustCompile(`(?is)<a\b[^>]*?epub:type="([^"]*)"[^>]*?href="([^"#]*)`)
	landmarkB = regexp.MustCompile(`(?is)<a\b[^>]*?href="([^"#]*)[^>]*?epub:type="([^"]*)"`)
	isbnWord  = regexp.MustCompile(`(?i)isbn(?:-?1[03])?[^0-9]{0,14}([0-9][0-9\- ]{8,16}[0-9Xx])`)
	isbnBare  = regexp.MustCompile(`\b97[89][0-9\- ]{10,14}[0-9]\b`)
	titleName = regexp.MustCompile(`(?i)title|titlepg|tp\d*\.|^tp\b`)
	copyName  = regexp.MustCompile(`(?i)copyright|copyr|imprint|legal|rights|colophon`)
	notTitle  = regexp.MustCompile(`(?i)subtitle|toc|contents|nav|cover`)
)

// Front reads the front matter of an EPUB.
func Front(file string) (FrontMatter, error) {
	var fm FrontMatter
	zr, err := zip.OpenReader(file)
	if err != nil {
		return fm, err
	}
	defer zr.Close()
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	var c container
	if err := readXML(files["META-INF/container.xml"], &c); err != nil || len(c.Rootfiles) == 0 {
		return fm, errors.New("the EPUB has no container.xml")
	}
	opfPath := c.Rootfiles[0].FullPath
	var o opfFull
	if err := readXML(files[opfPath], &o); err != nil {
		return fm, err
	}
	if len(o.Metadata.Titles) > 0 {
		fm.Title = strings.TrimSpace(o.Metadata.Titles[0])
	}
	if len(o.Metadata.Creators) > 0 {
		fm.Author = strings.TrimSpace(o.Metadata.Creators[0])
	}
	if len(o.Metadata.Publishers) > 0 {
		fm.Publisher = strings.TrimSpace(o.Metadata.Publishers[0])
	}
	for _, id := range o.Metadata.Identifiers {
		fm.ISBNs = addISBN(fm.ISBNs, id)
	}
	dir := path.Dir(opfPath)
	hrefOf := map[string]string{} // manifest id -> path in the zip
	for _, it := range o.Manifest {
		h, err := url.PathUnescape(it.Href)
		if err != nil {
			h = it.Href
		}
		hrefOf[it.ID] = path.Join(dir, h)
	}
	// where the book itself says its title page and copyright page are
	marked := map[string]string{} // "title" / "copyright" -> path in the zip
	mark := func(kind, base, href string) {
		href = strings.TrimSpace(href)
		if href == "" || marked[kind] != "" {
			return
		}
		if h, err := url.PathUnescape(href); err == nil {
			href = h
		}
		marked[kind] = path.Join(base, href)
	}
	for _, g := range o.Guide {
		switch strings.ToLower(g.Type) {
		case "title-page", "titlepage":
			mark("title", dir, g.Href)
		case "copyright-page", "copyright":
			mark("copyright", dir, g.Href)
		}
	}
	landmark := func(base, typ, href string) {
		switch strings.ToLower(strings.TrimSpace(typ)) {
		case "titlepage":
			mark("title", base, href)
		case "copyright-page":
			mark("copyright", base, href)
		}
	}
	for _, it := range o.Manifest {
		if !strings.Contains(it.Props, "nav") {
			continue
		}
		raw, err := readAll(files[hrefOf[it.ID]])
		if err != nil {
			continue
		}
		navDir := path.Dir(hrefOf[it.ID])
		for _, m := range landmarkA.FindAllStringSubmatch(string(raw), -1) {
			landmark(navDir, m[1], m[2])
		}
		for _, m := range landmarkB.FindAllStringSubmatch(string(raw), -1) {
			landmark(navDir, m[2], m[1])
		}
	}
	// the first sections in reading order
	type page struct{ id, zipPath, text string }
	var pages []page
	for _, ref := range o.Spine {
		if len(pages) >= frontSections {
			break
		}
		p := hrefOf[ref.IDRef]
		raw, err := readAll(files[p])
		if err != nil {
			continue
		}
		pages = append(pages, page{ref.IDRef, p, parseSection(string(raw)).Text})
	}
	used := map[int]bool{}
	named := func(kind, name string) bool {
		if kind == "title" {
			return titleName.MatchString(name) && !notTitle.MatchString(name)
		}
		return copyName.MatchString(name)
	}
	pick := func(kind string, byWords func(string) bool) string {
		for i, pg := range pages { // 1. where the book says it is
			if marked[kind] != "" && strings.EqualFold(pg.zipPath, marked[kind]) {
				used[i] = true
				return firstWords(pg.text, pageWords)
			}
		}
		for i, pg := range pages { // 2. what the file is called
			if !used[i] && named(kind, pg.id+" "+path.Base(pg.zipPath)) {
				used[i] = true
				return firstWords(pg.text, pageWords)
			}
		}
		for i, pg := range pages { // 3. what the page says
			if !used[i] && byWords != nil && byWords(pg.text) {
				used[i] = true
				return firstWords(pg.text, pageWords)
			}
		}
		return ""
	}
	fm.TitlePage = pick("title", nil)
	fm.CopyrightPage = pick("copyright", func(t string) bool {
		l := strings.ToLower(t)
		return strings.Contains(l, "all rights reserved") || strings.Contains(l, "©") || strings.Contains(l, "isbn")
	})
	var open strings.Builder
	for i, pg := range pages {
		if !used[i] && len(strings.Fields(pg.text)) > 30 { // a real page of text, not a dedication or an advertisement
			open.WriteString(pg.text + "\n")
			break
		}
	}
	fm.Opening = firstWords(open.String(), pageWords)
	for _, id := range ISBNsIn(fm.CopyrightPage + " " + fm.TitlePage) {
		fm.ISBNs = addISBN(fm.ISBNs, id)
	}
	return fm, nil
}

func firstWords(s string, n int) string {
	w := strings.Fields(s)
	if len(w) > n {
		w = w[:n]
	}
	return strings.Join(w, " ")
}

func addISBN(list []string, raw string) []string {
	d := cleanISBN(raw)
	if d == "" {
		return list
	}
	for _, x := range list {
		if x == d {
			return list
		}
	}
	return append(list, d)
}

// ISBNsIn finds the valid ISBNs written in a text ("ISBN 978-0-14-143951-8").
func ISBNsIn(text string) []string {
	var out []string
	for _, m := range isbnWord.FindAllStringSubmatch(text, -1) {
		out = addISBN(out, m[1])
	}
	for _, m := range isbnBare.FindAllString(text, -1) {
		out = addISBN(out, m)
	}
	return out
}

// cleanISBN returns the digits of a valid ISBN-10 or ISBN-13 ("urn:isbn:978-0-14-143951-8"), or "".
func cleanISBN(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(raw) {
		if (r >= '0' && r <= '9') || r == 'X' {
			b.WriteRune(r)
		}
	}
	d := b.String()
	switch {
	case len(d) == 13 && (strings.HasPrefix(d, "978") || strings.HasPrefix(d, "979")):
		sum := 0
		for i, r := range d[:12] {
			n := int(r - '0')
			if i%2 == 1 {
				n *= 3
			}
			sum += n
		}
		if (10-sum%10)%10 == int(d[12]-'0') {
			return d
		}
	case len(d) == 10:
		sum := 0
		for i, r := range d {
			n := int(r - '0')
			if r == 'X' {
				if i != 9 {
					return ""
				}
				n = 10
			}
			sum += n * (10 - i)
		}
		if sum%11 == 0 {
			return d
		}
	}
	return ""
}
