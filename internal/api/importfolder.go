package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/calibresrv"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The watch folder: another app drops a book (a folder with the e-book and some extra files, or a zip of one) into a folder; every
// few minutes NovelCheck looks, unpacks zips, adds the e-book to Calibre the way an upload does (checked with Hardcover), and then
// deletes what it took from (or moves it to "done"). Anything it could not add goes to "failed" with a notice. Switched on by an admin.

const (
	keyImportOn     = "import_enabled"
	keyImportDir    = "import_dir"
	keyImportKeep   = "import_keep" // "true": move finished ones to "done" instead of deleting them
	keyImportLast   = "import_last"
	importSettle    = time.Minute // a file untouched this long is finished being copied
	importEvery     = 3 * time.Minute
	maxUnzipped     = 2 << 30
	maxUnzipEntries = 5000
)

// importFormatOrder: when one book comes in several formats, the first of these is added (the others are extras).
var importFormatOrder = []string{".epub", ".kepub", ".azw3", ".mobi", ".azw", ".fb2", ".pdf", ".cbz"}

var importMu sync.Mutex // one pass at a time

func (s *Server) importDir() string {
	if d := strings.TrimSpace(s.Store.Setting(keyImportDir)); d != "" {
		return d
	}
	return filepath.Join(s.Cfg.DataDir, "to-process")
}

// ImportLoop looks at the folder every few minutes (when an admin switched it on).
func (s *Server) ImportLoop(ctx context.Context) {
	t := time.NewTicker(importEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if s.Store.SettingBool(keyImportOn) {
				s.importOnce(ctx)
			}
		}
	}
}

// importOnce goes through what is in the folder now. It returns a line saying what happened.
func (s *Server) importOnce(ctx context.Context) string {
	if !importMu.TryLock() {
		return "already looking"
	}
	defer importMu.Unlock()
	dir := s.importDir()
	if s.Store.Setting(store.KeyCalibreSrvURL) == "" {
		return s.importNote("not set up: connect the Calibre Content server first")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return s.importNote("can't use the folder: " + err.Error())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return s.importNote("can't read the folder: " + err.Error())
	}
	added, dup, failed := 0, 0, 0
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "failed" || name == "done" || e.Type()&fs.ModeSymlink != 0 {
			continue
		}
		path := filepath.Join(dir, name)
		if !settled(path) {
			continue
		}
		a, d, err := s.importEntry(ctx, path)
		added, dup = added+a, dup+d
		if err != nil {
			failed++
			s.Store.Notify("warning", "import", fmt.Sprintf("Couldn't add “%s” from the to-process folder: %v. It was moved to the failed folder.", name, err), "#/library")
			moveInto(filepath.Join(dir, "failed"), path)
			continue
		}
		if s.Store.SettingBool(keyImportKeep) {
			moveInto(filepath.Join(dir, "done"), path)
		} else {
			_ = os.RemoveAll(path)
		}
	}
	if added > 0 {
		s.syncSoon(nil) // NovelCheck reads Calibre again so the new books show up
	}
	return s.importNote(fmt.Sprintf("%d added, %d already in the library, %d failed", added, dup, failed))
}

func (s *Server) importNote(msg string) string {
	msg = time.Now().UTC().Format("2006-01-02 15:04") + " UTC · " + msg
	_ = s.Store.SetSetting(keyImportLast, msg)
	return msg
}

// settled reports whether nothing in path changed for a minute (so a copy in progress is left alone).
func settled(path string) bool {
	newest := time.Time{}
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err == nil {
			if info, err := d.Info(); err == nil && info.ModTime().After(newest) {
				newest = info.ModTime()
			}
		}
		return nil
	})
	return !newest.IsZero() && time.Since(newest) > importSettle
}

// importEntry adds the e-books found in one file or folder. It returns how many were added and how many were already there.
func (s *Server) importEntry(ctx context.Context, path string) (added, dup int, err error) {
	root := path
	if info, e := os.Lstat(path); e != nil {
		return 0, 0, e
	} else if !info.IsDir() {
		if strings.ToLower(filepath.Ext(path)) == ".zip" {
			tmp, e := os.MkdirTemp("", "novelcheck-import-")
			if e != nil {
				return 0, 0, e
			}
			defer os.RemoveAll(tmp)
			if e := unzipSafely(path, tmp); e != nil {
				return 0, 0, fmt.Errorf("the zip can't be opened: %w", e)
			}
			root = tmp
		} else if !ebookExts[ext(path)] {
			return 0, 0, errors.New("it isn't an e-book or a zip")
		}
	}
	books := chooseBooks(root)
	if len(books) == 0 {
		return 0, 0, errors.New("no e-book file in it")
	}
	cal := s.calibreClient()
	for _, file := range books {
		info, e := os.Stat(file)
		if e != nil || info.Size() > maxEbookBytes {
			return added, dup, fmt.Errorf("“%s” is missing or over 200 MB", filepath.Base(file))
		}
		data, e := os.ReadFile(file)
		if e != nil {
			return added, dup, e
		}
		id, e := cal.AddBook(ctx, filepath.Base(file), data)
		if errors.Is(e, calibresrv.ErrDuplicate) {
			dup++
			continue
		}
		if e != nil {
			return added, dup, fmt.Errorf("Calibre refused “%s”: %w", filepath.Base(file), e)
		}
		added++
		res := s.checkUpload(ctx, cal, id) // fixes the title and author where Hardcover is sure
		title, _ := res["title"].(string)
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		}
		s.Store.Notify("info", "import", "Added “"+title+"” from the to-process folder.", "#/library")
	}
	return added, dup, nil
}

// chooseBooks lists the e-books under root, one file per book: books are told apart by file name without the extension, and when
// one book has several formats the best is taken (importFormatOrder). The files that are not e-books are ignored.
func chooseBooks(root string) []string {
	best := map[string]string{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Type()&fs.ModeSymlink != 0 || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if !ebookExts[ext(p)] {
			return nil
		}
		key := strings.ToLower(strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())))
		if cur, ok := best[key]; !ok || formatRank(p) < formatRank(cur) {
			best[key] = p
		}
		return nil
	})
	out := make([]string, 0, len(best))
	for _, p := range best {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func formatRank(p string) int {
	for i, e := range importFormatOrder {
		if ext(p) == e {
			return i
		}
	}
	return len(importFormatOrder)
}

// moveInto moves path into dir (made if needed), under a free name.
func moveInto(dir, path string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("import: %v", err)
		return
	}
	dst := filepath.Join(dir, filepath.Base(path))
	for i := 2; ; i++ {
		if _, err := os.Lstat(dst); err != nil {
			break
		}
		dst = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), i, filepath.Ext(path)))
	}
	if err := os.Rename(path, dst); err != nil {
		log.Printf("import: move %s: %v", path, err)
	}
}
