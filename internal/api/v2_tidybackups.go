package api

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Tidy backups (Health → Backups): takes out of the Kindle backups what is clutter, and only what is surely safe:
//   - Amazon's dictionary files (big, protected, put back by the Kindle itself), anywhere;
//   - from a Kindle app's backup, a file whose identical copy (same name, same size, same SHA-256) is in the backup of the
//     same Kindle's KOReader plugin (the two are the same Kindle when they are linked: "Same Kindle as…").
// The page shows the list first; only the main admin can delete.

type tidyItem struct {
	Device string `json:"device"` // the backup's folder
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Why    string `json:"why"`
}

func sumOf(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

type diskFile struct {
	full string
	rel  string
	size int64
}

func walkFiles(root string) []diskFile {
	var out []diskFile
	_ = filepath.WalkDir(root, func(p string, e os.DirEntry, err error) error {
		if err == nil && !e.IsDir() && e.Type().IsRegular() && !strings.HasSuffix(p, ".part") {
			if fi, err := e.Info(); err == nil {
				rel, _ := filepath.Rel(root, p)
				out = append(out, diskFile{p, filepath.ToSlash(rel), fi.Size()})
			}
		}
		return nil
	})
	return out
}

// tidyPlan is what Tidy backups would remove, and the Kindles it could not judge because their reports are not linked.
func (s *Server) tidyPlan() (items []tidyItem, unlinked []string) {
	seen := map[string]bool{}
	add := func(device string, f diskFile, why string) {
		if !seen[f.full] {
			seen[f.full] = true
			items = append(items, tidyItem{device, f.rel, f.size, why})
		}
	}
	users, _ := s.Store.ListUsers()
	for _, u := range users {
		devices := s.Store.Devices(u.ID)
		links := s.Store.DeviceLinks(u.ID)
		group := func(name string) string {
			if l := links[name]; l != "" {
				return l
			}
			return name
		}
		for _, d := range devices {
			root := s.kindleDir(u.ID, d.Name)
			files := walkFiles(root)
			for _, f := range files {
				for _, part := range strings.Split(strings.ToLower(f.rel), "/") {
					if part == "dictionaries" {
						add(d.Name, f, "Amazon's dictionary files")
						break
					}
				}
			}
			if !strings.HasSuffix(d.Name, store.AppDeviceSuffix) {
				continue
			}
			// the Kindle app's backup: files the same Kindle's plugin backup holds identically
			index := map[string][]string{}
			plugin := false
			for _, o := range devices {
				if o.Name == d.Name || strings.HasSuffix(o.Name, store.AppDeviceSuffix) {
					continue
				}
				if group(o.Name) != group(d.Name) {
					continue
				}
				plugin = true
				for _, f := range walkFiles(s.kindleDir(u.ID, o.Name)) {
					k := path.Base(strings.ToLower(f.rel)) + "|" + strconv.FormatInt(f.size, 10)
					index[k] = append(index[k], f.full)
				}
			}
			if !plugin {
				if len(files) > 0 && len(devices) > 1 {
					unlinked = append(unlinked, d.Name)
				}
				continue
			}
			for _, f := range files {
				if seen[f.full] {
					continue
				}
				for _, other := range index[path.Base(strings.ToLower(f.rel))+"|"+strconv.FormatInt(f.size, 10)] {
					if a := sumOf(f.full); a != "" && a == sumOf(other) {
						add(d.Name, f, "the plugin's backup of this Kindle has the same file")
						break
					}
				}
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Size > items[j].Size })
	return items, unlinked
}

// handleTidyPreview: GET /api/admin/backups/tidy.
func (s *Server) handleTidyPreview(w http.ResponseWriter, r *http.Request) {
	items, unlinked := s.tidyPlan()
	var bytes int64
	for _, it := range items {
		bytes += it.Size
	}
	shown := items
	if len(shown) > 60 {
		shown = shown[:60]
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(items), "bytes": bytes, "items": shown, "unlinked": unlinked, "can_delete": auth.UserFrom(r).Owner})
}

// handleTidyRun: POST /api/admin/backups/tidy (the main admin): deletes what the preview listed, and empty folders.
func (s *Server) handleTidyRun(w http.ResponseWriter, r *http.Request) {
	if !auth.UserFrom(r).Owner {
		writeErr(w, http.StatusForbidden, "only the main admin can delete backup files")
		return
	}
	items, _ := s.tidyPlan()
	root := filepath.Join(s.Cfg.DataDir, "kindle-backup")
	var gone, bytes int64
	for _, u := range func() []store.User { us, _ := s.Store.ListUsers(); return us }() {
		for _, d := range s.Store.Devices(u.ID) {
			dir := s.kindleDir(u.ID, d.Name)
			for _, it := range items {
				full := filepath.Join(dir, filepath.FromSlash(it.Path))
				if it.Device == d.Name && strings.HasPrefix(full, root+string(filepath.Separator)) {
					if os.Remove(full) == nil {
						gone++
						bytes += it.Size
					}
				}
			}
		}
	}
	pruneEmpty(root) // folders left empty
	writeJSON(w, http.StatusOK, map[string]any{"removed": gone, "bytes": bytes})
}

// pruneEmpty removes empty folders under root (not root itself), deepest first.
func pruneEmpty(root string) {
	var dirs []string
	_ = filepath.WalkDir(root, func(p string, e os.DirEntry, err error) error {
		if err == nil && e.IsDir() && p != root {
			dirs = append(dirs, p)
		}
		return nil
	})
	for i := len(dirs) - 1; i >= 0; i-- {
		_ = os.Remove(dirs[i]) // fails when it isn't empty, which is what is wanted
	}
}
