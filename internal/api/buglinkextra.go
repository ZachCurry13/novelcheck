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

// writeUnmatched lists the e-reader books NovelCheck couldn't match to a library book (per person and device,
// with a few examples), so a matching problem shows without anyone reporting it.
func (s *Server) writeUnmatched(b *strings.Builder) {
	var rows []struct {
		User   string `db:"username"`
		Device string `db:"device"`
		Count  int    `db:"n"`
		Some   string `db:"some"`
	}
	_ = s.Store.DB.Select(&rows, `SELECT u.username, d.device, COUNT(*) AS n,
		(SELECT GROUP_CONCAT(x.t, ' | ') FROM (SELECT COALESCE(NULLIF(e.title, ''), e.filename) || COALESCE(' · ' || NULLIF(e.author, ''), '') AS t
			FROM device_books e WHERE e.user_id = d.user_id AND e.device = d.device AND e.book_id IS NULL AND e.ignored = 0 LIMIT 12) x) AS some
		FROM device_books d JOIN users u ON u.id = d.user_id WHERE d.book_id IS NULL AND d.ignored = 0
		GROUP BY d.user_id, d.device ORDER BY n DESC`)
	b.WriteString("\n== E-reader books NovelCheck couldn't match (person / device: how many, a few of them) ==\n")
	for _, r := range rows {
		fmt.Fprintf(b, "%s / %s: %d · %s\n", r.User, r.Device, r.Count, r.Some)
	}
	if len(rows) == 0 {
		b.WriteString("(none)\n")
	}
}
