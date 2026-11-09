package api

import (
	"context"
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
