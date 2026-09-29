// Package cli runs NovelCheck's maintenance commands, for the TrueNAS app's
// Shell (or `docker exec`): `novelcheck help` lists them. They work while the
// web app runs.
package cli

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/safemode"
	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/version"
)

var commands = [][2]string{
	{"users", "list the accounts"},
	{"reset-password <user> [password]", "give an account a new password (a random one if none is given) and sign it out everywhere"},
	{"backup [file]", "copy the database (default: backups/ in the data folder)"},
	{"check", "check the database for damage and show what's set up"},
	{"safe-mode on|off|status", "start without background work next time (or not)"},
	{"version", "show the version"},
}

// Run runs a command (args without the program name). handled is false when
// there is none, or it's "serve": then the web app starts.
func Run(args []string, dataDir string, out, errw io.Writer) (code int, handled bool) {
	if len(args) == 0 || args[0] == "serve" {
		return 0, false
	}
	c := &cmd{dataDir: dataDir, out: out}
	var err error
	switch args[0] {
	case "help", "-h", "--help":
		c.help()
	case "version", "--version":
		fmt.Fprintln(out, version.Version)
	case "safe-mode":
		err = c.safeMode(args[1:])
	case "users":
		err = c.withStore(c.users)
	case "reset-password":
		if len(args) < 2 {
			err = fmt.Errorf("usage: novelcheck reset-password <user> [password]")
		} else {
			err = c.withStore(func(st *store.Store) error { return c.resetPassword(st, args[1], strings.Join(args[2:], " ")) })
		}
	case "backup":
		err = c.withStore(func(st *store.Store) error { return c.backup(st, strings.Join(args[1:], " ")) })
	case "check":
		err = c.withStore(c.check)
	default:
		fmt.Fprintf(errw, "unknown command %q\n\n", args[0])
		c.out = errw
		c.help()
		return 2, true
	}
	if err != nil {
		fmt.Fprintln(errw, "error:", err)
		return 1, true
	}
	return 0, true
}

type cmd struct {
	dataDir string
	out     io.Writer
}

func (c *cmd) help() {
	fmt.Fprintf(c.out, "NovelCheck %s\n\nUsage: novelcheck <command>\n\n", version.Version)
	for _, cm := range commands {
		fmt.Fprintf(c.out, "  %-34s %s\n", cm[0], cm[1])
	}
	fmt.Fprintln(c.out, "\nWith no command (or `serve`), NovelCheck starts the web app.")
}

// withStore opens the existing database (never a new one).
func (c *cmd) withStore(fn func(*store.Store) error) error {
	path := filepath.Join(c.dataDir, "novelcheck.db")
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("no database at %s (is NOVELCHECK_DATA_DIR right?)", path)
	}
	d, err := db.Open(c.dataDir)
	if err != nil {
		return err
	}
	defer d.Close()
	return fn(store.New(d))
}

func (c *cmd) users(st *store.Store) error {
	us, err := st.ListUsers()
	if err != nil {
		return err
	}
	for _, u := range us {
		role := u.Role
		if role == store.RoleRestricted {
			role = "kid"
		}
		fmt.Fprintf(c.out, "%-24s %s\n", u.Username, role)
	}
	if len(us) == 0 {
		fmt.Fprintln(c.out, "No accounts yet: open NovelCheck in a browser to create the admin.")
	}
	return nil
}

func (c *cmd) resetPassword(st *store.Store, name, pw string) error {
	u, err := st.UserByName(name)
	if err != nil {
		var names []string
		us, _ := st.ListUsers()
		for _, x := range us {
			names = append(names, x.Username)
		}
		return fmt.Errorf("no account named %q (accounts: %s)", name, strings.Join(names, ", "))
	}
	if pw == "" {
		pw = randomPassword()
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		return err
	}
	if err := st.SetPassword(u.ID, hash); err != nil {
		return err
	}
	_ = st.DeleteUserSessions(u.ID)
	fmt.Fprintf(c.out, "The password for %s is now: %s\nSign in with it, then change it under Profile. %s was signed out on every device.\n", u.Username, pw, u.Username)
	return nil
}

// randomPassword is 12 easy-to-read letters and digits.
func randomPassword() string {
	const chars = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 12)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[n.Int64()]
	}
	return string(b)
}

func (c *cmd) backup(st *store.Store, target string) error {
	if target == "" {
		target = filepath.Join(c.dataDir, "backups", "novelcheck-"+time.Now().Format("20060102-150405")+".db")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%s already exists", target)
	}
	if _, err := st.DB.Exec(`VACUUM INTO ?`, target); err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.out, "Backed up to %s (%.1f MB). It holds passwords (scrambled) and API keys: keep it safe.\n", target, float64(info.Size())/1e6)
	return nil
}

func (c *cmd) safeMode(args []string) error {
	switch strings.Join(args, " ") {
	case "on":
		if err := safemode.SetFlag(c.dataDir, true); err != nil {
			return err
		}
		fmt.Fprintln(c.out, "Safe mode is on from the next start: restart the app. Leave it with the banner in the app or `novelcheck safe-mode off`.")
	case "off":
		if err := safemode.SetFlag(c.dataDir, false); err != nil {
			return err
		}
		safemode.Clean(c.dataDir)
		fmt.Fprintln(c.out, "Safe mode is off from the next start (restart the app, or press Leave safe mode in it).")
	case "", "status":
		state := "off"
		if safemode.FlagSet(c.dataDir) {
			state = "on (novelcheck safe-mode on)"
		}
		if v := os.Getenv(safemode.EnvVar); v != "" {
			state += fmt.Sprintf("; %s=%s", safemode.EnvVar, v)
		}
		fmt.Fprintln(c.out, "Safe mode:", state)
	default:
		return fmt.Errorf("usage: novelcheck safe-mode on|off|status")
	}
	return nil
}
