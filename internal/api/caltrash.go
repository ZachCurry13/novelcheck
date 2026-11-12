package api

import (
	"context"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Calibre's recycle bin: books removed from Calibre (by NovelCheck, Calibre or Calibre-Web) are moved to
// <library>/.caltrash/b/<calibre id>/ and kept there for a while. NovelCheck reads that folder so the owner
// can see what was removed and when, and put a book back: it is added to Calibre again from its files (as a
// new entry, with the title and authors it had).

type trashFile struct {
	Name   string `json:"name"`
	Format string `json:"format"`
	Size   int64  `json:"size"`
	path   string
}

type trashBook struct {
	ID      string      `json:"id"` // the Calibre id it had
	Title   string      `json:"title"`
	Author  string      `json:"author"`
	Removed string      `json:"removed"` // when it went into the bin (UTC)
	Files   []trashFile `json:"files"`
}

var (
	opfTitle   = regexp.MustCompile(`(?is)<dc:title[^>]*>(.*?)</dc:title>`)
	opfCreator = regexp.MustCompile(`(?is)<dc:creator[^>]*>(.*?)</dc:creator>`)
	notBooks   = map[string]bool{"opf": true, "json": true, "jpg": true, "jpeg": true, "png": true, "db": true, "": true}
)

func (s *Server) trashDir() string {
	if s.Syncer == nil || s.Syncer.LibraryDir() == "" {
		return ""
	}
	return filepath.Join(s.Syncer.LibraryDir(), ".caltrash", "b")
}

// readTrashBook reads one entry of Calibre's recycle bin (.caltrash/b/<id>/): the title and authors from its
// metadata.opf and its book files. Calibre keeps the files directly in the folder; one level of subfolders is
// looked into as well.
func readTrashBook(dir string, e os.DirEntry) trashBook {
	b := trashBook{ID: e.Name()}
	root := filepath.Join(dir, e.Name())
	if info, err := e.Info(); err == nil {
		b.Removed = info.ModTime().UTC().Format(time.RFC3339)
	}
	var look func(folder string, depth int)
	look = func(folder string, depth int) {
		items, err := os.ReadDir(folder)
		if err != nil {
			return
		}
		for _, d := range items {
			p := filepath.Join(folder, d.Name())
			if d.IsDir() {
				if depth < 1 {
					look(p, depth+1)
				}
				continue
			}
			ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(d.Name()), "."))
			if ext == "opf" && b.Title == "" {
				if raw, err := os.ReadFile(p); err == nil && len(raw) < 2<<20 {
					if m := opfTitle.FindSubmatch(raw); m != nil {
						b.Title = strings.TrimSpace(html.UnescapeString(string(m[1])))
					}
					var authors []string
					for _, m := range opfCreator.FindAllSubmatch(raw, 5) {
						authors = append(authors, strings.TrimSpace(html.UnescapeString(string(m[1]))))
					}
					b.Author = strings.Join(authors, " & ")
				}
			}
			if !notBooks[ext] {
				f := trashFile{Name: d.Name(), Format: strings.ToUpper(ext), path: p}
				if info, err := d.Info(); err == nil {
					f.Size = info.Size()
				}
				b.Files = append(b.Files, f)
			}
		}
	}
	look(root, 0)
	if b.Title == "" && len(b.Files) > 0 {
		b.Title = strings.TrimSuffix(b.Files[0].Name, filepath.Ext(b.Files[0].Name))
	}
	if b.Title == "" {
		b.Title = "(no title: Calibre entry #" + b.ID + ")"
	}
	return b
}

// readTrash lists the books in Calibre's recycle bin, the newest removal first. Folders are read eight at a
// time (the library is usually on a network share, where each read waits); progress reports how far it got.
func (s *Server) readTrash(progress func(done, total int)) ([]trashBook, bool) {
	dir := s.trashDir()
	entries, err := os.ReadDir(dir)
	if dir == "" || err != nil {
		return nil, false
	}
	var dirs []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		}
	}
	out := make([]trashBook, len(dirs))
	next := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				out[i] = readTrashBook(dir, dirs[i])
				mu.Lock()
				done++
				if progress != nil {
					progress(done, len(dirs))
				}
				mu.Unlock()
			}
		}()
	}
	for i := range dirs {
		next <- i
	}
	close(next)
	wg.Wait()
	sort.Slice(out, func(i, j int) bool { return out[i].Removed > out[j].Removed })
	return out, true
}

// handleCalibreTrash: GET /api/admin/calibre/trash. The bin is read in the background (see caltrash_cache.go);
// until that is done the answer is {"reading": true, "done", "total"} and the page asks again.
func (s *Server) handleCalibreTrash(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.trashListing(r.URL.Query().Get("fresh") == "1"))
}

// handleCalibreTrashRestore: POST /api/admin/calibre/trash/restore {"id": "804"} adds the book to Calibre
// again from its files in the bin: the EPUB (or the largest file) first, the other formats onto the new entry.
func (s *Server) handleCalibreTrashRestore(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	if _, err := strconv.Atoi(body.ID); err != nil {
		writeErr(w, http.StatusBadRequest, "which book?")
		return
	}
	var b *trashBook // only this book's folder is read, not the whole bin
	if dir := s.trashDir(); dir != "" {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				if e.IsDir() && e.Name() == body.ID {
					one := readTrashBook(dir, e)
					b = &one
				}
			}
		}
	}
	if b == nil || len(b.Files) == 0 {
		writeErr(w, http.StatusNotFound, "that book isn't in Calibre's recycle bin any more, or it had no book file")
		return
	}
	sort.SliceStable(b.Files, func(i, j int) bool { // EPUB first, then the larger file
		ei, ej := b.Files[i].Format == "EPUB", b.Files[j].Format == "EPUB"
		return ei != ej && ei || ei == ej && b.Files[i].Size > b.Files[j].Size
	})
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 10*time.Minute)
	defer cancel()
	c := s.calibreClient()
	newID := 0
	var notes []string
	for i, f := range b.Files {
		if f.Size > maxRestore {
			notes = append(notes, f.Format+" is too big to send from here")
			continue
		}
		data, err := os.ReadFile(f.path)
		if err != nil {
			notes = append(notes, f.Format+": "+err.Error())
			continue
		}
		if newID == 0 {
			if newID, err = c.AddBook(ctx, sanitizeFilename(f.Name), data); err != nil {
				writeErr(w, http.StatusBadGateway, "Calibre didn't take the book back: "+err.Error())
				return
			}
			continue
		}
		if err := c.AddFormat(ctx, newID, f.Format, f.Name, data); err != nil {
			notes = append(notes, f.Format+": "+err.Error())
		}
		_ = i
	}
	if newID == 0 {
		writeErr(w, http.StatusBadGateway, "nothing could be sent to Calibre: "+strings.Join(notes, "; "))
		return
	}
	// The title and authors it had (a file's own details can differ from what Calibre showed).
	var fields [][]any
	if !strings.HasPrefix(b.Title, "(no title") {
		fields = append(fields, []any{"title", b.Title})
	}
	if b.Author != "" {
		fields = append(fields, []any{"authors", strings.Split(b.Author, " & ")})
	}
	if len(fields) > 0 {
		if _, err := c.SetFields(ctx, newID, fields); err != nil {
			notes = append(notes, "its title and authors were not set: "+err.Error())
		}
	}
	go func() {
		s.refreshMu.Lock()
		defer s.refreshMu.Unlock()
		s.refreshCalibre()
	}()
	s.forgetTrash()
	writeJSON(w, http.StatusOK, map[string]any{"new_id": newID, "title": b.Title, "notes": notes})
}
