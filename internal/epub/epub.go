// Package epub reads the text of an EPUB file in reading order, section by
// section, for Deep read. It follows META-INF/container.xml to the package
// file's manifest and spine (EPUB 2 and 3) and needs no outside tools.
package epub

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// Section is one file of the book in reading order, usually a chapter.
type Section struct {
	Title string // its first heading, e.g. "Chapter 12" ("" if none)
	Text  string
}

// maxEntry skips absurdly large files inside the zip.
const maxEntry = 16 << 20

type container struct {
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type pkg struct {
	Manifest []struct {
		ID   string `xml:"id,attr"`
		Href string `xml:"href,attr"`
		Type string `xml:"media-type,attr"`
	} `xml:"manifest>item"`
	Spine []struct {
		IDRef string `xml:"idref,attr"`
	} `xml:"spine>itemref"`
}

// Read returns the book's sections in reading order.
func Read(file string) ([]Section, error) {
	zr, err := zip.OpenReader(file)
	if err != nil {
		return nil, fmt.Errorf("not a readable EPUB: %w", err)
	}
	defer zr.Close()
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	var c container
	if err := readXML(files["META-INF/container.xml"], &c); err != nil || len(c.Rootfiles) == 0 {
		return nil, errors.New("the EPUB has no container.xml")
	}
	opfPath := c.Rootfiles[0].FullPath
	var p pkg
	if err := readXML(files[opfPath], &p); err != nil {
		return nil, fmt.Errorf("the EPUB's package file is unreadable: %w", err)
	}
	items := map[string]int{}
	for i, it := range p.Manifest {
		items[it.ID] = i
	}
	var out []Section
	for _, ref := range p.Spine {
		i, ok := items[ref.IDRef]
		if !ok || !strings.Contains(p.Manifest[i].Type, "html") {
			continue
		}
		href, err := url.PathUnescape(p.Manifest[i].Href)
		if err != nil {
			href = p.Manifest[i].Href
		}
		raw, err := readAll(files[path.Join(path.Dir(opfPath), href)])
		if err != nil {
			continue // a missing chapter shouldn't sink the whole book
		}
		if s := parseSection(string(raw)); s.Text != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no readable text in this EPUB (it may be protected by DRM)")
	}
	return out, nil
}

// Words counts the words in all sections.
func Words(sections []Section) int {
	n := 0
	for _, s := range sections {
		n += len(strings.Fields(s.Text))
	}
	return n
}

func readAll(f *zip.File) ([]byte, error) {
	if f == nil {
		return nil, errors.New("missing file")
	}
	if f.UncompressedSize64 > maxEntry {
		return nil, errors.New("file too large")
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, maxEntry))
}

func readXML(f *zip.File, v any) error {
	raw, err := readAll(f)
	if err != nil {
		return err
	}
	return xml.Unmarshal(raw, v)
}

var (
	headRE    = regexp.MustCompile(`(?is)<head\b.*?</head>`)
	scriptRE  = regexp.MustCompile(`(?is)<script\b.*?</script>`)
	styleRE   = regexp.MustCompile(`(?is)<style\b.*?</style>`)
	headingRE = regexp.MustCompile(`(?is)<h[1-3]\b[^>]*>(.*?)</h[1-3]>`)
	blockRE   = regexp.MustCompile(`(?i)</?(p|div|br|h[1-6]|li|tr|section|blockquote)\b[^>]*>`)
	tagRE     = regexp.MustCompile(`<[^>]*>`)
	spaceRE   = regexp.MustCompile(`[ \t\r\f\v\x{00a0}]+`)
	blankRE   = regexp.MustCompile(`\s*\n\s*`)
)

// parseSection turns one XHTML file into plain text plus its first heading.
func parseSection(doc string) Section {
	for _, re := range []*regexp.Regexp{headRE, scriptRE, styleRE} {
		doc = re.ReplaceAllString(doc, " ")
	}
	var s Section
	if m := headingRE.FindStringSubmatch(doc); m != nil {
		s.Title = clean(tagRE.ReplaceAllString(m[1], " "))
		if r := []rune(s.Title); len(r) > 60 {
			s.Title = string(r[:59]) + "…"
		}
	}
	text := tagRE.ReplaceAllString(blockRE.ReplaceAllString(doc, "\n"), "")
	s.Text = strings.TrimSpace(blankRE.ReplaceAllString(spaceRE.ReplaceAllString(html.UnescapeString(text), " "), "\n"))
	return s
}

func clean(s string) string {
	return strings.TrimSpace(spaceRE.ReplaceAllString(html.UnescapeString(s), " "))
}
