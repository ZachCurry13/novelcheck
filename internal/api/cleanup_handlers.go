package api

import (
	"log"
	"net/http"

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
	dups, err := s.Store.ExtraEntries()
	if err != nil {
		writeStoreErr(w, err)
		return
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
	dupIDs := make([]string, 0, len(dups))
	for _, d := range dups {
		dupIDs = append(dupIDs, d.CalibreID)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"configured":   s.Store.Setting(store.KeyCalibreSrvURL) != "",
		"pairs":        len(pairs),
		"titles":       len(fixes),
		"title_ids":    ids,
		"dup_count":    len(dups),
		"dup_ids":      dupIDs,
		"dups":         dups[:min(len(dups), 200)],
		"convert":      len(convert),
		"remove_files": files,
		"remove_bytes": bytes,
		"job":          s.formats.state(),
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

// handleCleanupRefresh: POST re-reads the Calibre library and answers when it is done.
func (s *Server) handleCleanupRefresh(w http.ResponseWriter, r *http.Request) {
	s.refreshCalibre()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
