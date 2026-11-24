package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// More for the bug link: the family's own filters (their names and what the AI looks for) and each Kindle backup's
// size, newest file and unfinished pieces (a restore is only as good as the backup).

func (s *Server) writeFamilyFilters(b *strings.Builder) {
	flags, _ := s.Store.CustomFlags()
	fmt.Fprintf(b, "\n== The family's own filters (%d) ==\n", len(flags))
	for _, f := range flags {
		fmt.Fprintf(b, "%s · %q · looks for: %s\n", f.Key, f.Label, orNone(f.Description))
	}
}

func (s *Server) writeKindleBackups(b *strings.Builder) {
	root := filepath.Join(s.Cfg.DataDir, "kindle-backup")
	b.WriteString("\n== Kindle backups (user / device: files, size, newest file, unfinished pieces) ==\n")
	users, _ := os.ReadDir(root)
	for _, u := range users {
		devs, _ := os.ReadDir(filepath.Join(root, u.Name()))
		for _, d := range devs {
			files, parts := 0, 0
			var bytes int64
			var newest time.Time
			_ = filepath.WalkDir(filepath.Join(root, u.Name(), d.Name()), func(p string, e os.DirEntry, err error) error {
				if err != nil || e.IsDir() {
					return nil
				}
				if strings.HasSuffix(p, ".part") {
					parts++
					return nil
				}
				if fi, err := e.Info(); err == nil {
					files++
					bytes += fi.Size()
					if fi.ModTime().After(newest) {
						newest = fi.ModTime()
					}
				}
				return nil
			})
			fmt.Fprintf(b, "%s / %s: %d files, %.1f MB, newest %s UTC, %d unfinished\n", u.Name(), d.Name(), files, float64(bytes)/1e6,
				newest.UTC().Format("2006-01-02 15:04"), parts)
		}
	}
}
