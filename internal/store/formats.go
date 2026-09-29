package store

// Format cleanup: which formats of Calibre books a parent can remove (all
// but the ones they keep, only where a kept one exists), which books have no
// EPUB but a format Calibre can convert, and a log of removals for Undo.

import (
	"sort"
	"strings"
)

// CalibreFile is one file of a Calibre book.
type CalibreFile struct {
	BookID    int64  `db:"book_id" json:"book_id"`
	Title     string `db:"title" json:"title"`
	Author    string `db:"author" json:"author"`
	CalibreID string `db:"external_id" json:"calibre_id"`
	Format    string `db:"format" json:"format"`
	Path      string `db:"path" json:"-"`
	Size      int64  `db:"-" json:"size"` // set by the API, from the file
}

// FormatGroup is a Calibre book with the formats a cleanup keeps and removes.
type FormatGroup struct {
	BookID    int64         `json:"book_id"`
	Title     string        `json:"title"`
	Author    string        `json:"author"`
	CalibreID string        `json:"calibre_id"`
	Keep      []string      `json:"keep"`
	Remove    []CalibreFile `json:"remove"`
}

// Convertible formats: the ones Calibre turns into a good EPUB.
var Convertible = map[string]bool{"azw3": true, "mobi": true, "azw": true, "fb2": true, "docx": true, "rtf": true,
	"txt": true, "lit": true, "pdb": true, "htmlz": true, "odt": true}

// calibreFiles lists every file of the Calibre library, by book.
func (s *Store) calibreFiles() ([]CalibreFile, error) {
	var fs []CalibreFile
	err := s.DB.Select(&fs, `SELECT cb.book_id, b.title, b.author, cb.external_id, LOWER(cb.format) AS format, cb.path
		FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id AND c.source = 'calibre' JOIN books b ON b.id = cb.book_id
		WHERE cb.format NOT IN ('', 'list', 'paper') AND cb.external_id != '' ORDER BY b.title COLLATE NOCASE, cb.external_id`)
	return fs, err
}

// groupByEntry groups files by Calibre entry, keeping the order.
func groupByEntry(fs []CalibreFile) [][]CalibreFile {
	var out [][]CalibreFile
	at := map[string]int{}
	for _, f := range fs {
		i, ok := at[f.CalibreID]
		if !ok {
			i = len(out)
			at[f.CalibreID] = i
			out = append(out, nil)
		}
		out[i] = append(out[i], f)
	}
	return out
}

// FormatCleanup lists the Calibre books that have a kept format and others:
// the others would go. A book without any kept format is left alone.
func (s *Store) FormatCleanup(keep []string) ([]FormatGroup, error) {
	want := map[string]bool{}
	for _, k := range keep {
		want[strings.ToLower(strings.TrimSpace(k))] = true
	}
	fs, err := s.calibreFiles()
	if err != nil {
		return nil, err
	}
	out := []FormatGroup{}
	for _, entry := range groupByEntry(fs) {
		g := FormatGroup{BookID: entry[0].BookID, Title: entry[0].Title, Author: entry[0].Author, CalibreID: entry[0].CalibreID}
		for _, f := range entry {
			if want[f.Format] {
				g.Keep = append(g.Keep, strings.ToUpper(f.Format))
			} else {
				g.Remove = append(g.Remove, f)
			}
		}
		if len(g.Keep) > 0 && len(g.Remove) > 0 {
			out = append(out, g)
		}
	}
	return out, nil
}

// ToConvert lists the Calibre books with no EPUB and a format Calibre can
// convert (the best one to convert from first).
func (s *Store) ToConvert() ([]CalibreFile, error) {
	fs, err := s.calibreFiles()
	if err != nil {
		return nil, err
	}
	rank := map[string]int{"azw3": 0, "mobi": 1, "azw": 2, "docx": 3, "fb2": 4, "odt": 5, "htmlz": 6, "rtf": 7, "lit": 8, "pdb": 9, "txt": 10}
	out := []CalibreFile{}
	for _, entry := range groupByEntry(fs) {
		var best *CalibreFile
		hasEPUB := false
		for i, f := range entry {
			hasEPUB = hasEPUB || f.Format == "epub"
			if Convertible[f.Format] && (best == nil || rank[f.Format] < rank[best.Format]) {
				best = &entry[i]
			}
		}
		if !hasEPUB && best != nil {
			out = append(out, *best)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title) })
	return out, nil
}

// CalibreFormatCounts is how many Calibre books have each format.
func (s *Store) CalibreFormatCounts() (map[string]int, error) {
	var rows []struct {
		Format string `db:"format"`
		N      int    `db:"n"`
	}
	err := s.DB.Select(&rows, `SELECT UPPER(cb.format) AS format, COUNT(DISTINCT cb.external_id) AS n
		FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id AND c.source = 'calibre'
		WHERE cb.format NOT IN ('', 'list', 'paper') AND cb.external_id != '' GROUP BY UPPER(cb.format)`)
	out := map[string]int{}
	for _, r := range rows {
		out[r.Format] = r.N
	}
	return out, err
}
