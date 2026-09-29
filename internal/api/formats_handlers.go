package api

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Format cleanup (Admin → Calibre): keep the chosen formats of each Calibre
// book and remove the others through the Content server, with Undo for
// store.UndoDays days, and convert books with no EPUB. Admins only.

func splitFormats(v string) []string {
	var out []string
	for _, f := range strings.Split(v, ",") {
		if f = strings.ToUpper(strings.TrimSpace(f)); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// sized adds each file's size (for the preview) and totals them.
func (s *Server) sized(groups []store.FormatGroup) (files int, bytes int64) {
	for _, g := range groups {
		for i, f := range g.Remove {
			files++
			if st, err := os.Stat(f.Path); err == nil && s.insideCalibre(f.Path) {
				g.Remove[i].Size = st.Size()
				bytes += st.Size()
			}
		}
	}
	return
}

// handleFormats previews a cleanup (?keep=EPUB,PDF; the saved choice when
// empty) and lists books to convert, removals to undo and the job.
func (s *Server) handleFormats(w http.ResponseWriter, r *http.Request) {
	keep := splitFormats(r.URL.Query().Get("keep"))
	if len(keep) == 0 {
		keep = splitFormats(s.Store.Setting(store.KeyFormatKeep))
	}
	if len(keep) == 0 {
		keep = []string{"EPUB"}
	}
	groups, err := s.Store.FormatCleanup(keep)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	counts, err := s.Store.CalibreFormatCounts()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	convert, err := s.Store.ToConvert()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	batches, err := s.Store.RemovalBatches(100)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	files, bytes := s.sized(groups)
	writeJSON(w, http.StatusOK, map[string]any{
		"configured":    s.Store.Setting(store.KeyCalibreSrvURL) != "",
		"keep":          keep,
		"formats":       counts,
		"groups":        groups[:min(len(groups), 300)],
		"group_count":   len(groups),
		"file_count":    files,
		"bytes":         bytes,
		"convert":       convert[:min(len(convert), 300)],
		"convert_count": len(convert),
		"batches":       batches,
		"undo_days":     store.UndoDays,
		"job":           s.formats.state(),
	})
}

// formatJobReady refuses when the Content server isn't set up or a job runs.
func (s *Server) formatJobReady(w http.ResponseWriter) bool {
	if s.Store.Setting(store.KeyCalibreSrvURL) == "" {
		writeErr(w, http.StatusBadRequest, "set up the calibre Content server connection first")
		return false
	}
	if s.formats.state().Running {
		writeErr(w, http.StatusConflict, "a format job is already running; wait for it or stop it")
		return false
	}
	return true
}

// handleRemoveFormats starts the cleanup the admin reviewed (refused if the
// list changed since).
func (s *Server) handleRemoveFormats(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Keep     []string `json:"keep"`
		Expected int      `json:"expected_files"`
	}
	if !readJSON(w, r, &body, 8<<10) || !s.formatJobReady(w) {
		return
	}
	keep := splitFormats(strings.Join(body.Keep, ","))
	if len(keep) == 0 {
		writeErr(w, http.StatusBadRequest, "tick at least one format to keep")
		return
	}
	groups, err := s.Store.FormatCleanup(keep)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	files := 0
	for _, g := range groups {
		files += len(g.Remove)
	}
	if files == 0 || files != body.Expected {
		writeErr(w, http.StatusConflict, "the list changed since you reviewed it; please check it again")
		return
	}
	_ = s.Store.SetSetting(store.KeyFormatKeep, strings.Join(keep, ","))
	if !s.formats.begin("remove", files) {
		writeErr(w, http.StatusConflict, "a format job is already running")
		return
	}
	go s.runRemoval(groups, keep, auth.UserFrom(r).Username)
	writeJSON(w, http.StatusAccepted, s.formats.state())
}

// handleUndoFormats puts back one removed file ({"id"}) or a whole run ({"batch"}).
func (s *Server) handleUndoFormats(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID    int64  `json:"id"`
		Batch string `json:"batch"`
	}
	if !readJSON(w, r, &body, 4<<10) || !s.formatJobReady(w) {
		return
	}
	var items []store.FormatRemoval
	if body.ID > 0 {
		if it, err := s.Store.FormatRemovalByID(body.ID); err == nil {
			items = append(items, *it)
		}
	} else {
		var err error
		if items, err = s.Store.FormatRemovals(body.Batch, 0); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	if len(items) == 0 {
		writeErr(w, http.StatusNotFound, "nothing to put back: it's back already, or older than "+strconv.Itoa(store.UndoDays)+" days")
		return
	}
	if !s.formats.begin("restore", len(items)) {
		writeErr(w, http.StatusConflict, "a format job is already running")
		return
	}
	go s.runRestore(items)
	writeJSON(w, http.StatusAccepted, s.formats.state())
}

// handleConvertFormats converts the given books (all waiting when none) to EPUB.
func (s *Server) handleConvertFormats(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BookIDs []int64 `json:"book_ids"`
	}
	if !readJSON(w, r, &body, 64<<10) || !s.formatJobReady(w) {
		return
	}
	all, err := s.Store.ToConvert()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	pick := map[int64]bool{}
	for _, id := range body.BookIDs {
		pick[id] = true
	}
	var files []store.CalibreFile
	for _, f := range all {
		if len(pick) == 0 || pick[f.BookID] {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		writeErr(w, http.StatusBadRequest, "nothing to convert: those books have an EPUB already")
		return
	}
	if !s.formats.begin("convert", len(files)) {
		writeErr(w, http.StatusConflict, "a format job is already running")
		return
	}
	go s.runConvert(files)
	writeJSON(w, http.StatusAccepted, s.formats.state())
}

func (s *Server) handleFormatJob(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.formats.state())
}

// handleStopFormatJob stops the job after the file or book it's on.
func (s *Server) handleStopFormatJob(w http.ResponseWriter, r *http.Request) {
	s.formats.askStop()
	writeJSON(w, http.StatusOK, s.formats.state())
}
