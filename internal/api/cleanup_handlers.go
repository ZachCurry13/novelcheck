package api

import (
	"log"
	"net/http"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Library cleanup (Admin → Library cleanup): one place that shows what is left to tidy and
// runs the guarded steps in order. Each step uses the same safe tools as before (title fixes,
// format cleanup with Undo, the Content server), so a book is never changed without a check.

// handleCleanup is the summary: what each step would do now.
func (s *Server) handleCleanup(w http.ResponseWriter, r *http.Request) {
	pairs, _ := s.Store.SamePairs()
	fixes, err := s.Store.TitleFixes()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	dupGroups, err := s.Store.CalibreDuplicates()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	var dupIDs []string // the extra entries NovelCheck would remove: all but the best copy of each book
	for _, g := range dupGroups {
		keep := store.SuggestKeep(g.Entries)
		for _, e := range g.Entries {
			if e.CalibreID != keep {
				dupIDs = append(dupIDs, e.CalibreID)
			}
		}
	}
	convert, err := s.Store.ToConvert()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	groups, err := s.Store.FormatCleanup([]string{"EPUB"})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	files, bytes := s.sized(groups)
	ids := make([]int64, 0, len(fixes))
	for _, f := range fixes {
		ids = append(ids, f.ID)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"configured":     s.Store.Setting(store.KeyCalibreSrvURL) != "",
		"pairs":          len(pairs),
		"calibre_dups":   len(dupGroups),
		"titles":         len(fixes),
		"title_ids":      ids,
		"dup_count":      len(dupIDs),
		"convert":        len(convert),
		"remove_files":   files,
		"remove_bytes":   bytes,
		"job":            s.formats.state(),
		"convert_failed": len(s.Store.ConvertFailures()),
	})
}

// refreshCalibre re-reads the Calibre library now (when it can), so the lists match what Calibre has.
func (s *Server) refreshCalibre() {
	if s.Syncer != nil && s.Syncer.Available() {
		if _, err := s.Syncer.Run(); err != nil {
			log.Printf("calibre sync during library cleanup failed: %v", err)
		}
	}
}

// handleCleanupRefresh: POST re-reads the Calibre library. It answers when that is done, or after a minute
// with "running" (a big library takes longer than a tunnel waits); the reading carries on either way.
func (s *Server) handleCleanupRefresh(w http.ResponseWriter, r *http.Request) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.refreshMu.Lock() // one at a time: a second press waits for the first
		defer s.refreshMu.Unlock()
		s.refreshCalibre()
	}()
	select {
	case <-done:
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case <-time.After(time.Minute):
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true, "running": true})
	}
}
