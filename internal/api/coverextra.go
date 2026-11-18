package api

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/covers"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The cover finder's places for Kindle-only books (most free Kindle books have no ISBN, and Open Library and
// Google Books don't know them): the cover inside the book's own file, in Calibre or in the Kindle backup, then
// Amazon's cover picture by the book's ASIN.

var coverFileExts = map[string]bool{".epub": true, ".mobi": true, ".azw": true, ".azw3": true, ".prc": true}

// kindleBackupFiles maps each book file name in the Kindle backups (lower case) to where it is kept. It is made
// once per cover-finder run.
func (s *Server) kindleBackupFiles() map[string]string {
	out := map[string]string{}
	root := filepath.Join(s.Cfg.DataDir, "kindle-backup")
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && coverFileExts[strings.ToLower(filepath.Ext(p))] {
			if k := strings.ToLower(d.Name()); out[k] == "" {
				out[k] = p
			}
		}
		return nil
	})
	return out
}

// bookCoverFiles lists the book's own files that may hold its cover: Calibre's copies, then the Kindle backup's.
func (s *Server) bookCoverFiles(b store.BriefBook, copies []store.BookCopy, backup map[string]string) []string {
	var out []string
	for _, c := range copies {
		if c.Source == "calibre" && coverFileExts[strings.ToLower(filepath.Ext(c.Path))] && s.insideCalibre(c.Path) {
			if _, err := os.Stat(c.Path); err == nil {
				out = append(out, c.Path)
			}
		}
	}
	for _, n := range s.Store.DeviceFileNames(b.ID) {
		if p := backup[strings.ToLower(path.Base(strings.ReplaceAll(n, "\\", "/")))]; p != "" {
			out = append(out, p)
		}
	}
	return out
}

// bookASINs gathers the book's Amazon numbers: the one its free-book list gave, and any in its copies' ids and
// file names and in the names its Kindle files have.
func (s *Server) bookASINs(b store.BriefBook, copies []store.BookCopy) []string {
	texts := []string{b.ASIN}
	for _, c := range copies {
		texts = append(texts, c.ExternalID, c.Path)
	}
	texts = append(texts, s.Store.DeviceFileNames(b.ID)...)
	out := covers.ASINsIn(texts...)
	if a := isbn10(b.ASIN); a != "" {
		out = append(out, a)
	}
	return out
}

// extraCover tries the book's own files, then Amazon by ASIN, and says whether one gave a cover.
func (s *Server) extraCover(ctx context.Context, cv covers.Cache, b store.BriefBook, backup map[string]string) bool {
	copies, _ := s.Store.BookCopies(b.ID)
	name := s.coverName(b.ID)
	for _, f := range s.bookCoverFiles(b, copies, backup) {
		if cv.FromBookFile(name, f) {
			return true
		}
	}
	for _, a := range s.bookASINs(b, copies) {
		if cv.Amazon(ctx, name, a) {
			return true
		}
	}
	return false
}
