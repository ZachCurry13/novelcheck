package api

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/go-chi/chi/v5"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

// The database backups (backup.go) on the Health page: the list, "Back up now", and (the main admin only: the file holds
// everyone's password hashes) a download, so a copy can be kept somewhere else than the server.

var backupName = regexp.MustCompile(`^novelcheck-\d{4}-\d{2}-\d{2}\.db$`)

type backupFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	At   string `json:"at"`
}

func (s *Server) backupFiles() []backupFile {
	files, _ := filepath.Glob(filepath.Join(s.Cfg.DataDir, "backups", "novelcheck-*.db"))
	sort.Sort(sort.Reverse(sort.StringSlice(files))) // the date is in the name: newest first
	out := []backupFile{}
	for _, f := range files {
		if fi, err := os.Stat(f); err == nil && backupName.MatchString(filepath.Base(f)) {
			out = append(out, backupFile{filepath.Base(f), fi.Size(), fi.ModTime().UTC().Format("2006-01-02 15:04:05")})
		}
	}
	return out
}

// handleBackups: GET /api/admin/backups.
func (s *Server) handleBackups(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"files": s.backupFiles(), "can_download": auth.UserFrom(r).Owner})
}

// handleBackupNow: POST /api/admin/backups/now.
func (s *Server) handleBackupNow(w http.ResponseWriter, r *http.Request) {
	if err := s.BackupNow(); err != nil {
		writeErr(w, http.StatusInternalServerError, "the backup failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": s.backupFiles()})
}

// handleBackupFile: GET /api/admin/backups/{name}, the main admin's download of one copy.
func (s *Server) handleBackupFile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !auth.UserFrom(r).Owner {
		writeErr(w, http.StatusForbidden, "only the main admin can download a backup: it holds everyone's sign-in details")
		return
	}
	if !backupName.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeFile(w, r, filepath.Join(s.Cfg.DataDir, "backups", name))
}
