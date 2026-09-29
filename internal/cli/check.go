package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/safemode"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// check looks for database damage and says what's set up.
func (c *cmd) check(st *store.Store) error {
	var problems []string
	var rows []string
	if err := st.DB.Select(&rows, `PRAGMA integrity_check`); err != nil {
		return err
	}
	if len(rows) != 1 || rows[0] != "ok" {
		problems = append(problems, "database damage: "+strings.Join(rows, "; "))
	}
	var fk []struct {
		Table string `db:"table"`
	}
	if err := st.DB.Select(&fk, `SELECT "table" FROM pragma_foreign_key_check`); err == nil && len(fk) > 0 {
		problems = append(problems, fmt.Sprintf("%d rows point at missing rows (first in %s)", len(fk), fk[0].Table))
	}
	count := func(q string) int {
		var n int
		_ = st.DB.Get(&n, q)
		return n
	}
	fmt.Fprintf(c.out, "Database: %s\n", map[bool]string{true: "OK", false: "PROBLEMS"}[len(problems) == 0])
	fmt.Fprintf(c.out, "Accounts: %d (%d admins)\n", count(`SELECT COUNT(*) FROM users`), count(`SELECT COUNT(*) FROM users WHERE role = 'admin'`))
	fmt.Fprintf(c.out, "Books: %d, rated %d, failed %d\n", count(`SELECT COUNT(*) FROM books`),
		count(`SELECT COUNT(*) FROM books WHERE status = 'analyzed'`), count(`SELECT COUNT(*) FROM books WHERE status = 'error'`))
	ais := st.AIConfigs()
	if len(ais) == 0 {
		fmt.Fprintln(c.out, "AI: not set up (Admin → AI & Scans)")
	} else {
		fmt.Fprintf(c.out, "AI: %s at %s\n", strings.Join(ais[0].Models, ", "), orDash(ais[0].BaseURL))
	}
	fmt.Fprintf(c.out, "Calibre library folder: %s\n", orDash(st.Setting(store.KeyCalibreLibraryPath)))
	safe := "off"
	if safemode.FlagSet(c.dataDir) {
		safe = "on"
	}
	fmt.Fprintf(c.out, "Safe mode flag: %s\n", safe)
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(c.out, " -", p)
		}
		return errors.New("the database has problems; restore a backup (novelcheck backup makes one) or ask for help with the list above")
	}
	return nil
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(not set)"
	}
	return s
}
