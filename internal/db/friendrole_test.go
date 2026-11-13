package db

import (
	"strings"
	"testing"
)

// A database from before friends' accounts keeps its people, and its
// sessions still point at them, after the users table is rebuilt.
func TestAddFriendRoleKeepsEveryone(t *testing.T) {
	d, err := OpenDSN("file:" + t.TempDir() + "/old.db?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	// Make it look like v1.74: the old constraint, someone signed in.
	var ddl string
	_ = d.Get(&ddl, `SELECT sql FROM sqlite_master WHERE name = 'users'`)
	old := strings.Replace(strings.Replace(ddl, ", 'friend'", "", 1), "CREATE TABLE users", "CREATE TABLE users_old", 1)
	old = strings.Replace(old, "CREATE TABLE IF NOT EXISTS users", "CREATE TABLE users_old", 1)
	for _, q := range []string{`PRAGMA foreign_keys = OFF`, old, `DROP TABLE users`, `ALTER TABLE users_old RENAME TO users`, `PRAGMA foreign_keys = ON`,
		`INSERT INTO users (username, password_hash, role) VALUES ('mom', 'x', 'editor')`,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ('t1', 1, '2099-01-01')`} {
		if _, err := d.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if _, err := d.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('pal', 'x', 'friend')`); err == nil {
		t.Fatal("the old table took a friend")
	}
	if err := addFriendRole(d); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('pal', 'x', 'friend')`); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = d.Get(&n, `SELECT COUNT(*) FROM sessions s JOIN users u ON u.id = s.user_id WHERE u.username = 'mom'`)
	if n != 1 {
		t.Fatalf("mom's session: %d", n)
	}
	if err := addFriendRole(d); err != nil { // a second start changes nothing
		t.Fatal(err)
	}
}
