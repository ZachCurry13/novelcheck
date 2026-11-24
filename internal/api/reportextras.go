package api

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Parts of the bug report that explain what the cleanup tools did: the last format job (with the
// reasons conversions failed) and the titles still waiting to be tidied in Calibre.

func (s *Server) formatJobSection(line func(string, ...any)) {
	st := s.formats.state()
	line("\n== Last format job (convert, remove, restore; kept until the server restarts) ==")
	if st.Kind == "" {
		line("(none since the server started)")
		return
	}
	line("%s · running %v · %d of %d done · %d skipped · %d failed · %s", st.Kind, st.Running, st.Done, st.Total, st.Skipped, st.Failed, orNone(st.Note))
	for _, e := range st.Errors {
		line("  failed: %s", e)
	}
}

func (s *Server) tidyWaitingSection(line func(string, ...any)) {
	fixes, err := s.Store.TitleFixes()
	line("\n== Titles waiting to be tidied in Calibre ==")
	line("NovelCheck reads the Calibre folder “%s”; the Content server serves the library “%s” (%s)", orNone(s.Store.Setting("calibre_library_path")),
		orNone(s.Store.Setting("calibre_server_library")), orNone(s.Store.Setting("calibre_server_url")))
	if err != nil || len(fixes) == 0 {
		line("(none waiting)")
		return
	}
	shown := fixes[:min(len(fixes), 20)]
	var ids []int
	for _, f := range shown {
		ids = append(ids, s.Store.CalibreIDs(f.ID)...)
	}
	var onServer map[int]string
	if s.Store.Setting("calibre_server_url") != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		onServer, _ = s.calibreClient().Titles(ctx, ids)
	}
	line("%d waiting; the first %d:", len(fixes), len(shown))
	for _, f := range shown {
		parts := []string{}
		for _, id := range s.Store.CalibreIDs(f.ID) {
			t, ok := onServer[id]
			switch {
			case onServer == nil:
				parts = append(parts, "#"+strconv.Itoa(id))
			case !ok:
				parts = append(parts, "#"+strconv.Itoa(id)+" (the Content server has no such book)")
			case t == f.Title:
				parts = append(parts, "#"+strconv.Itoa(id)+" (the Content server ALREADY has the tidy title: NovelCheck is reading a different or older copy)")
			default:
				parts = append(parts, "#"+strconv.Itoa(id)+" (the Content server has “"+t+"”)")
			}
		}
		line("  book %d: NovelCheck read “%s” from Calibre, shows “%s”; Calibre entries %s", f.ID, f.CalibreTitle, f.Title, orNone(strings.Join(parts, ", ")))
	}
}

// readLibrary is the Calibre folder NovelCheck reads, as the admin set it.
func (s *Server) readLibrary() string {
	if p := s.Store.Setting("calibre_library_path"); p != "" {
		return p
	}
	return "the Calibre folder"
}

// calibreCompareSection puts NovelCheck's view of Calibre next to the Content server's: is it the same library?
// A sample of ids NovelCheck holds (the newest and every 40th) is asked of the Content server, and each id it
// lacks is checked on disk: a book whose files are gone is a leftover, one whose files exist is a book in the
// folder NovelCheck reads that the Content server doesn't serve (another library, or an older copy).
func (s *Server) calibreCompareSection(line func(string, ...any)) {
	line("\n== Calibre: NovelCheck's folder against the Content server ==")
	ids, total, maxID := s.Store.CalibreSample(80)
	dir := ""
	if s.Syncer != nil {
		dir = s.Syncer.LibraryDir()
	}
	line("NovelCheck holds %d Calibre books, the highest id #%d, from the folder %s", total, maxID, orNone(dir))
	if dir != "" {
		if fi, err := os.Stat(filepath.Join(dir, "metadata.db")); err == nil {
			line("its metadata.db was last written %s UTC (%d KB)", fi.ModTime().UTC().Format("2006-01-02 15:04:05"), fi.Size()/1024)
		} else {
			line("its metadata.db: %v", err)
		}
	}
	if s.Store.Setting("calibre_server_url") == "" || len(ids) == 0 {
		line("(no Content server set up, or no books)")
		return
	}
	have, missing, gone, onDisk, examples, err := s.compareCalibre(ids)
	if err != nil {
		line("the Content server: %v", err)
		return
	}
	line("of %d sampled ids the Content server has %d and lacks %d: %d with their files gone (leftovers NovelCheck should forget), %d still with files on disk (a different or older library?)", len(ids), len(have), missing, gone, onDisk)
	if len(examples) > 0 {
		line("e.g. %s", strings.Join(examples, ", "))
	}
}

// compareCalibre asks the Content server for these Calibre ids: how many it has, lacks, lacks with no files on disk
// (leftovers NovelCheck should forget) and lacks while their files are on disk (it is behind, or serves another
// library), with a few examples.
func (s *Server) compareCalibre(ids []int) (have map[int]string, missing, gone, onDisk int, examples []string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if have, err = s.calibreClient().Titles(ctx, ids); err != nil {
		return nil, 0, 0, 0, nil, err
	}
	for _, id := range ids {
		if _, ok := have[id]; ok {
			continue
		}
		missing++
		files := s.Store.CalibreEntryFiles(id)
		exist := 0
		for _, p := range files {
			if _, err := os.Stat(p); err == nil {
				exist++
			}
		}
		if exist == 0 {
			gone++
		} else {
			onDisk++
		}
		if len(examples) < 6 {
			examples = append(examples, "#"+strconv.Itoa(id)+" ("+strconv.Itoa(exist)+" of "+strconv.Itoa(len(files))+" files on disk)")
		}
	}
	return have, missing, gone, onDisk, examples, nil
}

// calibreBehindMsg is what to do when the Content server lacks books whose files are in the library.
const calibreBehindMsg = "Calibre's Content server is behind your library: it doesn't have some of your newer books yet, though their files are there (books added with Calibre-Web or another program aren't seen until it reads the library again). Until then NovelCheck can't remove duplicates of them or tidy their titles. Restart the Calibre app (it reads the library when it starts)."

// checkCalibreBehind compares a sample of the library with the Content server and keeps a 🔔 notice while it is
// behind (cleared when it catches up). It reports whether it is behind.
func (s *Server) checkCalibreBehind() bool {
	if s.Store.Setting("calibre_server_url") == "" {
		return false
	}
	ids, _, _ := s.Store.CalibreSample(80)
	if len(ids) == 0 {
		return false
	}
	_, _, _, onDisk, _, err := s.compareCalibre(ids)
	if err != nil {
		return false
	}
	if onDisk > 0 {
		s.Store.Notify("warning", "calibre-behind", calibreBehindMsg, "#/admin?tab=cleanup")
		return true
	}
	s.Store.Resolve("calibre-behind")
	return false
}

// CalibreBehindLoop checks every hour (the first time ten minutes after start).
func (s *Server) CalibreBehindLoop(ctx context.Context) {
	wait := 10 * time.Minute
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = time.Hour
		s.checkCalibreBehind()
	}
}
