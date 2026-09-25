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
	if err := addEditorRole(d); err != nil {
		return err
	}
	if err := addColumn(d, "users", "guide_seen", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := addColumn(d, "sessions", "remember", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	if err := addColumn(d, "books", "approved", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	for _, c := range [][3]string{
		{"books", "approved_by", "TEXT NOT NULL DEFAULT ''"},
		{"books", "age_level", "INTEGER NOT NULL DEFAULT 0"},
		{"books", "age_set_by", "TEXT NOT NULL DEFAULT ''"},
		{"users", "age_level", "INTEGER NOT NULL DEFAULT 0"},
		{"users", "max_spice", "INTEGER NOT NULL DEFAULT -1"},
		{"books", "spice_level", "INTEGER"},
		{"books", "spice_reason", "TEXT NOT NULL DEFAULT ''"},
		{"books", "rules_version", "INTEGER NOT NULL DEFAULT 0"},
		{"books", "flags_version", "INTEGER NOT NULL DEFAULT 0"},
		{"books", "rated_modified", "TEXT NOT NULL DEFAULT ''"},
		{"catalog_books", "modified", "TEXT NOT NULL DEFAULT ''"},
		{"token_usage", "cost", "REAL"},
	} {
		if err := addColumn(d, c[0], c[1], c[2]); err != nil {
			return err
		}
	}
	return nil
}

// addColumn adds a column if an older database doesn't have it yet.
func addColumn(d *sqlx.DB, table, column, def string) error {
	var n int
	if err := d.Get(&n, `SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := d.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, def))
	return err
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
