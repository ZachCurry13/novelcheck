package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

// handleDAVTidy removes the empty folders in the person's WebDAV folder: the
// "start folders" KOReader asked for that nothing was ever saved in. Files are
// never touched, and neither is the folder itself.
func (s *Server) handleDAVTidy(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	root := s.davDir(u.ID)
	var dirs []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() && p != root {
			dirs = append(dirs, p)
		}
		return nil
	})
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) }) // deepest first
	removed := 0
	for _, d := range dirs {
		if os.Remove(d) == nil { // refuses a folder that holds anything
			removed++
		}
	}
	writeJSON(w, http.StatusOK, map[string]int{"removed": removed})
}
