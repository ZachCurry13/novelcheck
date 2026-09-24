package api

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/calibre"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Bounds for the automatic library search so huge media shares stay fast.
const (
	findMaxDepth = 5
	findMaxDirs  = 5000
)

// handleCalibreBrowse lists subfolders of ?path= (relative to the mount).
func (s *Server) handleCalibreBrowse(w http.ResponseWriter, r *http.Request) {
	l, err := calibre.Browse(s.Cfg.CalibreDir, r.URL.Query().Get("path"))
	if err != nil {
		writeBrowseErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"mount":            s.Cfg.CalibreDir,
		"selected":         s.Store.Setting(store.KeyCalibreLibraryPath),
		"library":          s.Syncer.LibraryDir(),
		"library_is_valid": s.Syncer.Available(),
		"listing":          l,
	})
}

// handleCalibreFind searches the mount for folders containing metadata.db.
func (s *Server) handleCalibreFind(w http.ResponseWriter, r *http.Request) {
	found, err := calibre.FindLibraries(s.Cfg.CalibreDir, findMaxDepth, findMaxDirs)
	if err != nil {
		writeBrowseErr(w, err)
		return
	}
	if found == nil {
		found = []calibre.Folder{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"mount": s.Cfg.CalibreDir, "libraries": found})
}

// handleSetCalibreLibrary saves the chosen library folder and re-syncs.
func (s *Server) handleSetCalibreLibrary(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	rel, err := s.Syncer.SetLibrary(body.Path)
	if err != nil {
		writeBrowseErr(w, err)
		return
	}
	go func() {
		if _, err := s.Syncer.Run(); err != nil {
			log.Printf("calibre sync after library change failed: %v", err)
		}
	}()
	writeJSON(w, http.StatusOK, map[string]string{"path": rel, "library": s.Syncer.LibraryDir()})
}

func writeBrowseErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, calibre.ErrOutsideMount):
		writeErr(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, os.ErrNotExist):
		writeErr(w, http.StatusNotFound, "folder not found; check the Calibre volume in your app settings")
	case errors.Is(err, os.ErrPermission):
		writeErr(w, http.StatusForbidden, "permission denied; give the apps user read access to this folder")
	default:
		writeErr(w, http.StatusBadRequest, err.Error())
	}
}

// handleCalibreRemoval lists the Calibre books that match the given hide
// filters (?hide=nudity,dark_occult&classification=...), and builds a Calibre
// search that selects exactly those books so the admin can remove them in
// Calibre itself. Parent-approved books are never included. NovelCheck never
// deletes files: the Calibre library stays read-only.
func (s *Server) handleCalibreRemoval(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.BookFilter{
		Query:          q.Get("q"),
		Classification: q.Get("classification"),
		AnyFlags:       splitCSV(q.Get("hide")),
	}
	if len(f.AnyFlags) == 0 && f.Classification == "" {
		writeErr(w, http.StatusBadRequest, "tick at least one Hide box (or pick a spice level) first")
		return
	}
	matches, err := s.Store.CalibreMatches(f)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if matches == nil {
		matches = []store.CalibreMatch{}
	}
	parts := make([]string, 0, len(matches))
	for _, m := range matches {
		parts = append(parts, "id:="+m.CalibreID)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count":  len(matches),
		"books":  matches,
		"search": strings.Join(parts, " or "),
		// true when the Content server is set up for one-click removal
		"one_click": s.Store.Setting(store.KeyCalibreSrvURL) != "",
	})
}
