package api

import (
	"context"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// handleAdminImport: GET shows the watch folder's settings and what the last look did; PUT {"enabled", "dir", "keep"} changes them.
func (s *Server) handleAdminImport(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		var body struct {
			Enabled bool   `json:"enabled"`
			Dir     string `json:"dir"`
			Keep    bool   `json:"keep"`
		}
		if !readJSON(w, r, &body, 2<<10) {
			return
		}
		dir := strings.TrimSpace(body.Dir)
		if dir != "" && (!filepath.IsAbs(dir) || len(dir) > 300) {
			writeErr(w, http.StatusBadRequest, "the folder must be a full path, like /data/to-process")
			return
		}
		if dir == filepath.Join(s.Cfg.DataDir, "to-process") {
			dir = "" // the default
		}
		for k, v := range map[string]string{keyImportOn: strconv.FormatBool(body.Enabled), keyImportDir: dir, keyImportKeep: strconv.FormatBool(body.Keep)} {
			if err := s.Store.SetSetting(k, v); err != nil {
				writeStoreErr(w, err)
				return
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": s.Store.SettingBool(keyImportOn), "dir": s.importDir(),
		"keep": s.Store.SettingBool(keyImportKeep), "last": s.Store.Setting(keyImportLast)})
}

// handleAdminImportNow: POST looks at the folder now (in the background; the answer is the "last" line of GET).
func (s *Server) handleAdminImportNow(w http.ResponseWriter, r *http.Request) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		s.importOnce(ctx)
	}()
	writeJSON(w, http.StatusAccepted, map[string]bool{"started": true})
}
