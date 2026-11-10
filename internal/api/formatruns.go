package api

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/calibresrv"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The format jobs themselves. Every one checks calibre's titles first, so a
// Content server serving a different library is never touched.

// maxRestore is the biggest file Undo sends back to calibre (it travels in
// one request); bigger ones are restored in calibre itself.
const maxRestore = 400 << 20

// trashFile finds a removed format in calibre's recycle bin
// (<library>/.caltrash/f/<calibre id>/<format>), or "".
func (s *Server) trashFile(calibreID, format string) string {
	dir := filepath.Join(s.Syncer.LibraryDir(), ".caltrash", "f", calibreID)
	lower := strings.ToLower(format)
	for _, p := range []string{filepath.Join(dir, lower), filepath.Join(dir, "*."+lower)} {
		if ms, _ := filepath.Glob(p); len(ms) > 0 {
			if st, err := os.Stat(ms[0]); err == nil && st.Mode().IsRegular() {
				return ms[0]
			}
		}
	}
	return ""
}

// checkTitles asks calibre for the books. A book whose title isn't the one NovelCheck has is left out (and
// noted on the job), so one renamed book doesn't hold up the rest; when more than one in five differ it is the
// wrong library, and the job stops.
func (s *Server) checkTitles(c *calibresrv.Client, want map[int]string) (map[int]calibresrv.Book, error) {
	s.formats.update(func(st *formatJobState) { st.Current = "Checking the books with Calibre…" })
	ids := make([]int, 0, len(want))
	for id := range want {
		ids = append(ids, id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	have, err := c.Books(ctx, ids)
	if err != nil {
		return nil, err
	}
	var odd []int
	for id, title := range want {
		if b, ok := have[id]; ok && !sameCalibreTitle(b.Title, title) {
			odd = append(odd, id)
		}
	}
	if len(odd)*5 > len(want) {
		id := odd[0]
		return nil, fmt.Errorf("calibre's book #%d is %q but NovelCheck expected %q, and so on for %d of %d books. Is the Content server using the same library?", id, have[id].Title, want[id], len(odd), len(want))
	}
	for _, id := range odd {
		s.formats.fail(want[id], fmt.Sprintf("Calibre calls book #%d %q; left alone. Read the Calibre library again (Library cleanup) and retry.", id, have[id].Title))
		delete(have, id)
	}
	return have, nil
}

// afterFormatJob re-reads the library so the page shows the result.
func (s *Server) afterFormatJob() {
	if s.Syncer.Available() {
		if _, err := s.Syncer.Run(); err != nil {
			log.Printf("calibre sync after format job failed: %v", err)
		}
	}
	// The job ran in the background: say how it ended, so nobody has to watch the page.
	st := s.formats.state()
	if st.Kind == "" {
		return
	}
	msg := fmt.Sprintf("%s finished: %s", strings.ToUpper(st.Kind[:1])+st.Kind[1:], st.Note)
	if st.Failed > 0 {
		s.Store.Notify("warning", "format-job", fmt.Sprintf("%s %d did not work; the reasons are in the bug report (Settings → Developer).", msg, st.Failed), "#/admin?tab=cleanup")
		return
	}
	s.Store.NotifyRoutine("format-job", msg+" Next: Admin → Library cleanup.", "#/admin?tab=cleanup")
}

func mb(n int64) string { return strconv.FormatFloat(float64(n)/(1<<20), 'f', 1, 64) + " MB" }

// runRemoval removes the formats a cleanup doesn't keep, only from books
// calibre confirms still have a kept format.
func (s *Server) runRemoval(groups []store.FormatGroup, keep []string, by string) {
	defer s.afterFormatJob()
	c, j := s.calibreClient(), &s.formats
	want := map[int]string{}
	for _, g := range groups {
		if id, err := strconv.Atoi(g.CalibreID); err == nil {
			want[id] = g.Title
		}
	}
	have, err := s.checkTitles(c, want)
	if err != nil {
		j.end("Stopped: " + err.Error() + " Nothing was removed.")
		return
	}
	batch := time.Now().UTC().Format("2006-01-02 15:04:05")
	var freed int64
	checked := false
	for _, g := range groups {
		if j.stopped() {
			break
		}
		id, _ := strconv.Atoi(g.CalibreID)
		b, found := have[id]
		j.update(func(st *formatJobState) { st.Current = g.Title })
		kept := false
		for _, k := range keep {
			kept = kept || b.Has(k)
		}
		if !found || !kept { // gone from calibre, or its kept format isn't really there
			j.update(func(st *formatJobState) { st.Skipped += len(g.Remove) })
			continue
		}
		if why := s.keptLooksWrong(g); why != "" { // never remove the original when the copy that stays is no good
			j.fail(g.Title, why+" Its other files were kept.")
			continue
		}
		for _, f := range g.Remove {
			if !b.Has(f.Format) {
				j.update(func(st *formatJobState) { st.Skipped++ })
				continue
			}
			var size int64
			if st, err := os.Stat(f.Path); err == nil && s.insideCalibre(f.Path) {
				size = st.Size()
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			err := c.RemoveFormat(ctx, id, f.Format)
			cancel()
			if err != nil {
				j.fail(g.Title+" ("+strings.ToUpper(f.Format)+")", err.Error())
				continue
			}
			if err := s.Store.LogFormatRemoval(f, filepath.Base(f.Path), size, batch, by); err != nil {
				log.Printf("format cleanup: logging a removal failed: %v", err)
			}
			freed += size
			j.update(func(st *formatJobState) { st.Done++ })
			if !checked { // Undo needs calibre to keep what it removes
				checked = true
				if !s.waitForTrash(g.CalibreID, f.Format) {
					j.end("Stopped after one file: calibre removed " + strings.ToUpper(f.Format) + " of “" + g.Title +
						"” but it isn't in calibre's recycle bin (.caltrash in the library folder), so Undo couldn't bring files back. " +
						"Check NovelCheck reads the same library folder calibre uses.")
					return
				}
			}
		}
	}
	st := j.state()
	j.end(fmt.Sprintf("Removed %s (%s). Undo is under Removed formats for %d days.", count(st.Done, "file"), mb(freed), store.UndoDays))
	log.Printf("format cleanup: removed %d files via Content server (%s)", st.Done, by)
}

// keptLooksWrong checks the file that would be the book's only copy afterwards: it must be there, be more
// than a stub, and (an EPUB) be a real EPUB file. "" means it is fine.
func (s *Server) keptLooksWrong(g store.FormatGroup) string {
	for _, f := range g.KeepFiles {
		if !s.insideCalibre(f.Path) {
			continue
		}
		st, err := os.Stat(f.Path)
		if err != nil || st.Size() < 2048 {
			continue
		}
		if f.Format == "epub" {
			head := make([]byte, 4)
			fh, err := os.Open(f.Path)
			if err != nil {
				continue
			}
			_, err = io.ReadFull(fh, head)
			fh.Close()
			if err != nil || string(head[:2]) != "PK" {
				continue
			}
		}
		return ""
	}
	return "the " + strings.Join(g.Keep, "/") + " that would stay is missing, empty or not a real file, so nothing was removed from this book."
}

// waitForTrash gives a network share a moment to show calibre's new file.
func (s *Server) waitForTrash(calibreID, format string) bool {
	for i := 0; i < 6; i++ {
		if s.trashFile(calibreID, format) != "" {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// runRestore puts removed formats back from calibre's recycle bin.
func (s *Server) runRestore(items []store.FormatRemoval) {
	defer s.afterFormatJob()
	c, j := s.calibreClient(), &s.formats
	want := map[int]string{}
	for _, it := range items {
		if id, err := strconv.Atoi(it.CalibreID); err == nil {
			want[id] = it.Title
		}
	}
	have, err := s.checkTitles(c, want)
	if err != nil {
		j.end("Stopped: " + err.Error() + " Nothing was put back.")
		return
	}
	for _, it := range items {
		if j.stopped() {
			break
		}
		name := it.Title + " (" + it.Format + ")"
		j.update(func(st *formatJobState) { st.Current = name })
		id, _ := strconv.Atoi(it.CalibreID)
		b, found := have[id]
		switch {
		case !found:
			j.fail(name, "the book isn't in calibre any more")
			continue
		case b.Has(it.Format): // back already
			_ = s.Store.MarkRestored(it.ID)
			j.update(func(st *formatJobState) { st.Skipped++ })
			continue
		}
		path := s.trashFile(it.CalibreID, it.Format)
		if path == "" {
			j.fail(name, "calibre's recycle bin doesn't have it any more")
			continue
		}
		if st, err := os.Stat(path); err == nil && st.Size() > maxRestore {
			j.fail(name, "too big to send back from here; restore it in calibre (Remove books → Restore recently deleted)")
			continue
		}
		data, err := os.ReadFile(path)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			err = c.AddFormat(ctx, id, it.Format, it.FileName, data)
			cancel()
		}
		if err != nil {
			j.fail(name, err.Error())
			continue
		}
		_ = s.Store.MarkRestored(it.ID)
		j.update(func(st *formatJobState) { st.Done++ })
	}
	st := j.state()
	j.end("Put back " + count(st.Done, "file") + ".")
}
