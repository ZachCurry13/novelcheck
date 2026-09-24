package calibre

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The container mounts a parent folder at Dir (e.g. TrueNAS "red14/plex" at
// /calibre). The admin picks the actual library inside it from the web UI;
// that relative subfolder (e.g. "books/Clean Library") is stored in settings.

var ErrOutsideMount = errors.New("folder is outside the mounted Calibre directory")

// ResolveRel turns a user-supplied relative folder into an absolute path that
// is guaranteed (after resolving symlinks) to stay inside mount.
func ResolveRel(mount, rel string) (string, error) {
	rel = filepath.Clean("/" + strings.TrimSpace(rel)) // anchor, collapses any ".."
	abs := filepath.Join(mount, rel)
	realMount, err := filepath.EvalSymlinks(mount)
	if err != nil {
		return "", err
	}
	realAbs, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	r, err := filepath.Rel(realMount, realAbs)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", ErrOutsideMount
	}
	return abs, nil
}

// RelPath expresses abs relative to mount using forward slashes ("" = root).
func RelPath(mount, abs string) string {
	r, err := filepath.Rel(mount, abs)
	if err != nil || r == "." {
		return ""
	}
	return filepath.ToSlash(r)
}

// HasLibrary reports whether dir directly contains a Calibre metadata.db.
func HasLibrary(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "metadata.db"))
	return err == nil && st.Mode().IsRegular()
}

// Folder is one entry in the in-app folder browser.
type Folder struct {
	Name       string `json:"name"`
	Path       string `json:"path"` // relative to the mount
	HasLibrary bool   `json:"has_library"`
}

// Listing is the content of one browsed folder.
type Listing struct {
	Path       string   `json:"path"`
	Parent     *string  `json:"parent"` // nil at the mount root
	HasLibrary bool     `json:"has_library"`
	Folders    []Folder `json:"folders"`
	Truncated  bool     `json:"truncated"`
}

const maxListing = 500

// Browse lists the subfolders of rel (relative to mount), flagging any that
// are Calibre libraries. Dot-folders are skipped.
func Browse(mount, rel string) (*Listing, error) {
	abs, err := ResolveRel(mount, rel)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	out := &Listing{Path: RelPath(mount, abs), HasLibrary: HasLibrary(abs), Folders: []Folder{}}
	if out.Path != "" {
		p := RelPath(mount, filepath.Dir(abs))
		out.Parent = &p
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		child := filepath.Join(abs, e.Name())
		if st, err := os.Stat(child); err != nil || !st.IsDir() { // follows symlinked dirs
			continue
		}
		if len(out.Folders) >= maxListing {
			out.Truncated = true
			break
		}
		out.Folders = append(out.Folders, Folder{Name: e.Name(), Path: RelPath(mount, child), HasLibrary: HasLibrary(child)})
	}
	sort.Slice(out.Folders, func(i, j int) bool {
		return strings.ToLower(out.Folders[i].Name) < strings.ToLower(out.Folders[j].Name)
	})
	return out, nil
}

// FindLibraries searches below mount (breadth-first, bounded) for folders
// containing metadata.db. It does not descend into libraries it finds.
func FindLibraries(mount string, maxDepth, maxDirs int) ([]Folder, error) {
	if _, err := os.Stat(mount); err != nil {
		return nil, err
	}
	type item struct {
		path  string
		depth int
	}
	var found []Folder
	queue := []item{{mount, 0}}
	for visited := 0; len(queue) > 0 && visited < maxDirs; visited++ {
		cur := queue[0]
		queue = queue[1:]
		if HasLibrary(cur.path) {
			found = append(found, Folder{Name: filepath.Base(cur.path), Path: RelPath(mount, cur.path), HasLibrary: true})
			continue
		}
		if cur.depth >= maxDepth {
			continue
		}
		entries, err := os.ReadDir(cur.path)
		if err != nil {
			continue // unreadable folder: skip, keep searching
		}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				queue = append(queue, item{filepath.Join(cur.path, e.Name()), cur.depth + 1})
			}
		}
	}
	return found, nil
}

// LibraryDir returns the absolute library folder currently selected in
// settings, falling back to the mount root if the setting is invalid.
func (s *Syncer) LibraryDir() string {
	if abs, err := ResolveRel(s.Dir, s.Store.Setting(store.KeyCalibreLibraryPath)); err == nil {
		return abs
	}
	return s.Dir
}

// SetLibrary validates rel, saves it, and re-syncs from the new location.
// Copies from the previous library are dropped first so book ids from two
// different Calibre libraries can't be confused; books present in both keep
// their existing analysis because they match on title + author.
func (s *Syncer) SetLibrary(rel string) (string, error) {
	abs, err := ResolveRel(s.Dir, rel)
	if err != nil {
		return "", err
	}
	if !HasLibrary(abs) {
		return "", errors.New("no Calibre library (metadata.db) in that folder")
	}
	rel = RelPath(s.Dir, abs)
	s.mu.Lock()
	err = s.Store.SetSetting(store.KeyCalibreLibraryPath, rel)
	if err == nil {
		err = s.Store.ClearCatalogCopies(store.CalibreCatalogName)
	}
	s.mu.Unlock()
	return rel, err
}
