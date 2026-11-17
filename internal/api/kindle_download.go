package api

import (
	"archive/zip"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// The admin's way to get the copied Kindle books back out as they are (protected ones too): a list of what a Kindle
// holds, one file at a time, or the whole Kindle as one ZIP. Nothing is converted or unlocked.

// kindleRoot is the folder of one Kindle's copy, "" when the names do not point at one.
func (s *Server) kindleRoot(r *http.Request) string {
	user, device := r.URL.Query().Get("user"), r.URL.Query().Get("device")
	if _, err := strconv.ParseInt(user, 10, 64); err != nil || device == "" || strings.ContainsAny(device, "/\\") || device == ".." || device == "." {
		return ""
	}
	root := filepath.Join(s.Cfg.DataDir, "kindle-backup", user, device)
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return ""
	}
	return root
}

// handleKindleFiles: GET /api/admin/kindle-backup/files?user=&device= lists every copied file.
func (s *Server) handleKindleFiles(w http.ResponseWriter, r *http.Request) {
	root := s.kindleRoot(r)
	if root == "" {
		writeErr(w, http.StatusNotFound, "no such Kindle copy")
		return
	}
	type file struct {
		Path   string `json:"path"`
		Size   int64  `json:"size"`
		Locked bool   `json:"locked"`
	}
	out := []file{}
	_ = filepath.WalkDir(root, func(p string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() || strings.HasSuffix(p, ".part") || len(out) >= 5000 {
			return nil
		}
		fi, err := e.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		out = append(out, file{Path: filepath.ToSlash(rel), Size: fi.Size(), Locked: kindleLocked(p)})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Path) < strings.ToLower(out[j].Path) })
	writeJSON(w, http.StatusOK, map[string]any{"files": out})
}

// handleKindleFile: GET /api/admin/kindle-backup/file?user=&device=&path= sends one file as it was on the Kindle.
func (s *Server) handleKindleFile(w http.ResponseWriter, r *http.Request) {
	root := s.kindleRoot(r)
	rel := kindleRel(r.URL.Query().Get("path"))
	if root == "" || rel == "" {
		writeErr(w, http.StatusNotFound, "no such file")
		return
	}
	full := filepath.Join(root, filepath.FromSlash(rel))
	f, err := os.Open(full)
	if err != nil {
		writeErr(w, http.StatusNotFound, "no such file")
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || fi.IsDir() {
		writeErr(w, http.StatusNotFound, "no such file")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(path.Base(rel), `"`, "")+`"`)
	http.ServeContent(w, r, path.Base(rel), fi.ModTime(), f)
}

// handleKindleZip: GET /api/admin/kindle-backup/zip?user=&device= streams the whole copy as a ZIP (stored, not
// squeezed: e-books are compressed already, and it starts at once).
func (s *Server) handleKindleZip(w http.ResponseWriter, r *http.Request) {
	root := s.kindleRoot(r)
	if root == "" {
		writeErr(w, http.StatusNotFound, "no such Kindle copy")
		return
	}
	name := strings.ReplaceAll(r.URL.Query().Get("device"), `"`, "")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+` books.zip"`)
	zw := zip.NewWriter(w)
	defer zw.Close()
	_ = filepath.WalkDir(root, func(p string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() || strings.HasSuffix(p, ".part") || r.Context().Err() != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		fi, err := e.Info()
		if err != nil {
			return nil
		}
		h, _ := zip.FileInfoHeader(fi)
		h.Name, h.Method = filepath.ToSlash(rel), zip.Store
		dst, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		src, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer src.Close()
		_, err = io.Copy(dst, src)
		return err
	})
}
