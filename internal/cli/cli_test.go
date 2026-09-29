package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func run(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	var out, errw bytes.Buffer
	code, handled := Run(args, dir, &out, &errw)
	if !handled {
		t.Fatalf("%v not handled", args)
	}
	return out.String() + errw.String(), code
}

func TestCommands(t *testing.T) {
	dir := t.TempDir()
	if _, handled := Run(nil, dir, &bytes.Buffer{}, &bytes.Buffer{}); handled {
		t.Fatal("no command must start the web app")
	}
	if out, code := run(t, dir, "users"); code != 1 || !strings.Contains(out, "no database") {
		t.Fatalf("no database: %d %s", code, out)
	}
	d, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(d)
	hash, _ := auth.HashPassword("oldpassword")
	u, _ := st.CreateUser("parent", hash, store.RoleAdmin)
	d.Close()

	if out, code := run(t, dir, "users"); code != 0 || !strings.Contains(out, "parent") || !strings.Contains(out, "admin") {
		t.Fatalf("users: %d %s", code, out)
	}
	if out, code := run(t, dir, "reset-password", "nobody"); code != 1 || !strings.Contains(out, "accounts: parent") {
		t.Fatalf("unknown user: %d %s", code, out)
	}
	out, code := run(t, dir, "reset-password", "parent")
	if code != 0 {
		t.Fatalf("reset: %d %s", code, out)
	}
	pw := strings.Fields(strings.SplitN(out, "now: ", 2)[1])[0]
	d, _ = db.Open(dir)
	got, _ := store.New(d).UserByName("parent")
	if !auth.CheckPassword(got.PasswordHash, pw) || got.ID != u.ID {
		t.Fatalf("new password %q doesn't work", pw)
	}
	d.Close()

	if out, code := run(t, dir, "check"); code != 0 || !strings.Contains(out, "Database: OK") {
		t.Fatalf("check: %d %s", code, out)
	}
	target := filepath.Join(dir, "copy.db")
	if out, code := run(t, dir, "backup", target); code != 0 {
		t.Fatalf("backup: %d %s", code, out)
	}
	if info, err := os.Stat(target); err != nil || info.Size() == 0 {
		t.Fatalf("backup file: %v", err)
	}
	if _, code := run(t, dir, "safe-mode", "on"); code != 0 {
		t.Fatal("safe-mode on")
	}
	if out, _ := run(t, dir, "safe-mode", "status"); !strings.Contains(out, "on") {
		t.Fatalf("status: %s", out)
	}
	if out, code := run(t, dir, "frobnicate"); code != 2 || !strings.Contains(out, "reset-password") {
		t.Fatalf("unknown command: %d %s", code, out)
	}
}
