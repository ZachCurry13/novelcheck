package db

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

// migrate upgrades databases created by older versions. schema.sql uses
// CREATE TABLE IF NOT EXISTS, so constraint changes on existing tables must
// be applied here.
func migrate(d *sqlx.DB) error {
	return addEditorRole(d)
}

// addEditorRole widens users.role to allow 'editor' (added in v1.1). SQLite
// can't alter a CHECK constraint, so the table is rebuilt with foreign keys
// temporarily off (otherwise dropping users would cascade to sessions and
// queues).
func addEditorRole(d *sqlx.DB) error {
	var ddl string
	if err := d.Get(&ddl, `SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'users'`); err != nil {
		return err
	}
	if strings.Contains(ddl, "'editor'") {
		return nil
	}
	newDDL := strings.Replace(ddl, "CHECK (role IN ('admin', 'restricted'))",
		"CHECK (role IN ('admin', 'editor', 'restricted'))", 1)
	if newDDL == ddl {
		return fmt.Errorf("users table has an unexpected role constraint; cannot migrate")
	}
	newDDL = strings.Replace(newDDL, "CREATE TABLE users", "CREATE TABLE users_new", 1)
	newDDL = strings.Replace(newDDL, "CREATE TABLE IF NOT EXISTS users", "CREATE TABLE users_new", 1)

	if _, err := d.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer d.Exec(`PRAGMA foreign_keys = ON`)
	tx, err := d.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		newDDL,
		`INSERT INTO users_new SELECT * FROM users`,
		`DROP TABLE users`,
		`ALTER TABLE users_new RENAME TO users`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("migrate users table: %w", err)
		}
	}
	return tx.Commit()
}
