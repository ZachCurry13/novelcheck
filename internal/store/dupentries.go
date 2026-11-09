package store

import "strings"

// ExtraEntry is a Calibre entry that repeats a book another Calibre entry already has as an EPUB,
// and has no EPUB itself (for example a MOBI added again next to the EPUB).
type ExtraEntry struct {
	BookID    int64    `json:"book_id"`
	Title     string   `json:"title"`
	Author    string   `json:"author"`
	CalibreID string   `json:"calibre_id"`
	Formats   []string `json:"formats"`
	KeptID    string   `json:"kept_id"` // the Calibre entry with the EPUB that stays
}

// ExtraEntries lists the extra Calibre entries of a book (NovelCheck joins entries
// with the same title and author into one book) that have no EPUB while another entry has.
// Entries that both have an EPUB are left alone.
func (s *Store) ExtraEntries() ([]ExtraEntry, error) {
	fs, err := s.calibreFiles()
	if err != nil {
		return nil, err
	}
	byBook := map[int64][][]CalibreFile{}
	var order []int64
	for _, e := range groupByEntry(fs) {
		b := e[0].BookID
		if _, ok := byBook[b]; !ok {
			order = append(order, b)
		}
		byBook[b] = append(byBook[b], e)
	}
	hasEpub := func(e []CalibreFile) bool {
		for _, f := range e {
			if strings.EqualFold(f.Format, "epub") {
				return true
			}
		}
		return false
	}
	out := []ExtraEntry{}
	for _, b := range order {
		entries := byBook[b]
		if len(entries) < 2 {
			continue
		}
		kept := ""
		for _, e := range entries {
			if hasEpub(e) {
				kept = e[0].CalibreID
				break
			}
		}
		if kept == "" {
			continue
		}
		for _, e := range entries {
			if hasEpub(e) {
				continue
			}
			d := ExtraEntry{BookID: b, Title: e[0].Title, Author: e[0].Author, CalibreID: e[0].CalibreID, KeptID: kept}
			for _, f := range e {
				d.Formats = append(d.Formats, strings.ToUpper(f.Format))
			}
			out = append(out, d)
		}
	}
	return out, nil
}
