package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Restore from NovelCheck (the plugin's Kindle care menu): the plugin asks for what NovelCheck holds of this
// e-reader's backup and downloads the files that are missing on the e-reader, one at a time. Only the person's own
// backup; nothing on the e-reader is ever replaced (the plugin skips files that exist).

// handleRestoreList: GET /kosync/plugin/restore?device=, every file in this e-reader's backup with its size.
func (s *Server) handleRestoreList(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	root := s.kindleDir(u.ID, r.URL.Query().Get("device"))
	type file struct {
		Path string `json:"path"`
		Size int64  `json:"size"`
	}
	out := []file{}
	_ = filepath.WalkDir(root, func(p string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() || strings.HasSuffix(p, ".part") || e.Type()&os.ModeSymlink != 0 {
			return nil
		}
		fi, err := e.Info()
		rel, rerr := filepath.Rel(root, p)
		if err == nil && rerr == nil {
			out = append(out, file{filepath.ToSlash(rel), fi.Size()})
		}
		return nil
	})
	kosyncJSON(w, http.StatusOK, map[string]any{"files": out})
}

// handleRestoreFile: GET /kosync/plugin/restore/file?device=&path=, one file of this e-reader's backup.
func (s *Server) handleRestoreFile(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	rel := kindleRel(r.URL.Query().Get("path"))
	if rel == "" {
		http.NotFound(w, r)
		return
	}
	root := s.kindleDir(u.ID, r.URL.Query().Get("device"))
	p := filepath.Join(root, filepath.FromSlash(rel))
	if fi, err := os.Lstat(p); err != nil || !fi.Mode().IsRegular() || !strings.HasPrefix(p, root+string(filepath.Separator)) {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, p)
}
