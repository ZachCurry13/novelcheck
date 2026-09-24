package store

import (
	"sort"
	"strings"
)

// DupEntry is one Calibre entry (one Calibre book id) with its files.
type DupEntry struct {
	CalibreID string    `json:"calibre_id"`
	Files     []DupFile `json:"files"`
}

// DupFile is one format of a Calibre entry.
type DupFile struct {
	Format string `json:"format"`
	Path   string `json:"path,omitempty"`
	Size   int64  `json:"size"` // bytes on disk, 0 if unknown
}

// DupGroup is a book that appears as 2+ separate entries in Calibre (same
// title and author surname), e.g. imported twice.
type DupGroup struct {
	BookID  int64      `json:"book_id"`
	Title   string     `json:"title"`
	Author  string     `json:"author"`
	Entries []DupEntry `json:"entries"`
}

// CalibreDuplicates lists books with more than one Calibre entry.
func (s *Store) CalibreDuplicates() ([]DupGroup, error) {
	var rows []struct {
		BookID     int64  `db:"book_id"`
		Title      string `db:"title"`
		Author     string `db:"author"`
		ExternalID string `db:"external_id"`
		Format     string `db:"format"`
		Path       string `db:"path"`
	}
	err := s.DB.Select(&rows, `SELECT b.id AS book_id, b.title, b.author, cb.external_id, cb.format, cb.path
		FROM catalog_books cb JOIN catalogs c ON c.id = cb.catalog_id AND c.source = 'calibre'
		JOIN books b ON b.id = cb.book_id
		WHERE cb.book_id IN (SELECT d.book_id FROM catalog_books d JOIN catalogs dc ON dc.id = d.catalog_id
			AND dc.source = 'calibre' GROUP BY d.book_id HAVING COUNT(DISTINCT d.external_id) > 1)
		ORDER BY b.title COLLATE NOCASE, b.id, CAST(cb.external_id AS INTEGER), cb.format`)
	if err != nil {
		return nil, err
	}
	var out []DupGroup
	for _, r := range rows {
		if len(out) == 0 || out[len(out)-1].BookID != r.BookID {
			out = append(out, DupGroup{BookID: r.BookID, Title: r.Title, Author: r.Author})
		}
		g := &out[len(out)-1]
		if len(g.Entries) == 0 || g.Entries[len(g.Entries)-1].CalibreID != r.ExternalID {
			g.Entries = append(g.Entries, DupEntry{CalibreID: r.ExternalID, Files: []DupFile{}})
		}
		if r.Format != "" {
			e := &g.Entries[len(g.Entries)-1]
			e.Files = append(e.Files, DupFile{Format: r.Format, Path: r.Path})
		}
	}
	return out, nil
}

// formatScore ranks formats for "which copy to keep": EPUB is the most
// useful (Send-to-Kindle, most readers), then AZW3/KFX, MOBI, PDF.
var formatScore = map[string]int{"epub": 8, "azw3": 4, "kfx": 4, "azw": 3, "mobi": 2, "pdf": 1}

// SuggestKeep picks the entry to keep: best formats, then most files, then
// largest total size, then the oldest (lowest) Calibre id.
func SuggestKeep(entries []DupEntry) string {
	type scored struct {
		id              string
		fmt, files, idx int
		size            int64
	}
	var ss []scored
	for i, e := range entries {
		sc := scored{id: e.CalibreID, files: len(e.Files), idx: i}
		for _, f := range e.Files {
			sc.fmt += formatScore[strings.ToLower(f.Format)]
			sc.size += f.Size
		}
		ss = append(ss, sc)
	}
	sort.SliceStable(ss, func(a, b int) bool {
		x, y := ss[a], ss[b]
		if x.fmt != y.fmt {
			return x.fmt > y.fmt
		}
		if x.files != y.files {
			return x.files > y.files
		}
		if x.size != y.size {
			return x.size > y.size
		}
		return x.idx < y.idx
	})
	if len(ss) == 0 {
		return ""
	}
	return ss[0].id
}
