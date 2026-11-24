package api

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// A copy of NovelCheck's database every day (the first one a few minutes after start), kept for a week, in the data
// folder's backups. Ratings, Nightstands, settings and reading all live in that one file, so this is what a bad update or
// a failing disk can't take away. VACUUM INTO writes a clean, consistent copy while the app runs.

const keepBackups = 7

// BackupLoop makes the daily copies until ctx ends.
func (s *Server) BackupLoop(ctx context.Context) {
	wait := 3 * time.Minute
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		if at, ok := s.lastBackup(); !ok || time.Since(at) > 22*time.Hour {
			if err := s.BackupNow(); err != nil {
				log.Printf("backup of the database failed: %v", err)
			}
		}
		wait = time.Hour
	}
}

// BackupNow writes today's copy (replacing one made earlier today) and removes the ones past the last week.
func (s *Server) BackupNow() error {
	dir := filepath.Join(s.Cfg.DataDir, "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	out := filepath.Join(dir, "novelcheck-"+time.Now().UTC().Format("2006-01-02")+".db")
	tmp := out + ".part"
	_ = os.Remove(tmp)
	if _, err := s.Store.DB.Exec(fmt.Sprintf("VACUUM INTO '%s'", tmp)); err != nil { // the path is ours, never a person's text
		return err
	}
	if err := os.Rename(tmp, out); err != nil {
		return err
	}
	files, _ := filepath.Glob(filepath.Join(dir, "novelcheck-*.db"))
	sort.Strings(files) // the date in the name sorts them oldest first
	for len(files) > keepBackups {
		_ = os.Remove(files[0])
		files = files[1:]
	}
	return nil
}
