package covers

import (
	"os"
	"path/filepath"
	"regexp"
)

var storedName = regexp.MustCompile(`^[a-z0-9-]{1,80}$`)

// Stored returns a cover kept earlier under name (e.g. one fetched from Audiobookshelf).
func (c Cache) Stored(name string) (string, os.FileInfo, bool) {
	if c.Dir == "" || !storedName.MatchString(name) {
		return "", nil, false
	}
	p := filepath.Join(c.Dir, name+".jpg")
	info, err := os.Stat(p)
	return p, info, err == nil
}

// Keep saves a cover under name and returns it like Stored.
func (c Cache) Keep(name string, jpg []byte) (string, os.FileInfo, bool) {
	if c.Dir == "" || !storedName.MatchString(name) || len(jpg) < 200 || os.MkdirAll(c.Dir, 0o755) != nil {
		return "", nil, false
	}
	p := filepath.Join(c.Dir, name+".jpg")
	if os.WriteFile(p, jpg, 0o644) != nil {
		return "", nil, false
	}
	info, err := os.Stat(p)
	return p, info, err == nil
}

// SaveUpload keeps a picture someone uploaded (JPEG or PNG, at least 100 px a side, 8 MB at most) under name.
func (c Cache) SaveUpload(name string, data []byte) bool {
	return c.Dir != "" && storedName.MatchString(name) && c.saveImage(name, data)
}

// Drop removes a cover kept under name.
func (c Cache) Drop(name string) {
	if c.Dir != "" && storedName.MatchString(name) {
		_ = os.Remove(filepath.Join(c.Dir, name+".jpg"))
	}
}
