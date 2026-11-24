package api

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/config"
	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The daily backup is a real, readable copy of the database, and only the newest week is kept.
func TestBackupMakesAReadableCopy(t *testing.T) {
	d, err := db.OpenDSN(fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=foreign_keys(1)", t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	if _, err := st.UpsertBook("Mort", "Terry Pratchett", "", ""); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	s := &Server{Store: st, Cfg: config.Config{DataDir: dir}}
	for i := 1; i <= 9; i++ { // older days: only the newest week stays
		_ = os.MkdirAll(filepath.Join(dir, "backups"), 0o755)
		_ = os.WriteFile(filepath.Join(dir, "backups", fmt.Sprintf("novelcheck-2020-01-%02d.db", i)), []byte("old"), 0o644)
	}
	if err := s.BackupNow(); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "backups", "novelcheck-*.db"))
	if len(files) != keepBackups {
		t.Fatalf("%d copies kept", len(files))
	}
	if _, ok := s.lastBackup(); !ok {
		t.Fatal("the new copy isn't seen")
	}
	raw, err := os.ReadFile(files[len(files)-1]) // open a working copy of the backup (opening runs the migrations, which write)
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(t.TempDir(), "restored.db")
	if err := os.WriteFile(work, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	copyDB, err := db.OpenDSN("file:" + work)
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	var n int
	if err := copyDB.Get(&n, `SELECT COUNT(*) FROM books WHERE title = 'Mort'`); err != nil || n != 1 {
		t.Fatalf("the copy doesn't hold the book: %d %v", n, err)
	}
}
