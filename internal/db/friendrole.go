package db

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jmoiron/sqlx"
)

var tableName = regexp.MustCompile(`^CREATE TABLE (?:IF NOT EXISTS )?["\x60\[]?users["\x60\]]?`)

// addFriendRole widens users.role to allow 'friend' (v1.75: an adult from
// outside the family who never sees the family's libraries). Like
// addEditorRole, the table is rebuilt (SQLite can't change a CHECK) with
// foreign keys off, keeping every row and column as it is.
func addFriendRole(d *sqlx.DB) error {
	var ddl string
	if err := d.Get(&ddl, `SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'users'`); err != nil {
		return err
	}
	if strings.Contains(ddl, "'friend'") {
		return nil
	}
	const old = "CHECK (role IN ('admin', 'editor', 'restricted'))"
	if !strings.Contains(ddl, old) {
		return fmt.Errorf("users table has an unexpected role constraint; cannot add friends' accounts")
	}
	newDDL := strings.Replace(ddl, old, "CHECK (role IN ('admin', 'editor', 'restricted', 'friend'))", 1)
	// A table renamed by an earlier rebuild is stored as CREATE TABLE "users".
	newDDL = tableName.ReplaceAllString(newDDL, "CREATE TABLE users_new")
	if _, err := d.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer d.Exec(`PRAGMA foreign_keys = ON`)
	tx, err := d.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{newDDL, `INSERT INTO users_new SELECT * FROM users`, `DROP TABLE users`, `ALTER TABLE users_new RENAME TO users`} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("add friends' accounts to the users table: %w", err)
		}
	}
	return tx.Commit()
}
