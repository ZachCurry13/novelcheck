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
		{"books", "series", "TEXT NOT NULL DEFAULT ''"},
		{"books", "series_index", "REAL NOT NULL DEFAULT 0"},
		{"books", "title_fix", "TEXT NOT NULL DEFAULT ''"},
		{"books", "tags", "TEXT NOT NULL DEFAULT ''"},
		{"books", "genres", "TEXT NOT NULL DEFAULT ''"},
		{"books", "kind", "TEXT NOT NULL DEFAULT ''"},
		{"books", "genre_source", "TEXT NOT NULL DEFAULT ''"},
		{"suggestion_votes", "reason", "TEXT NOT NULL DEFAULT ''"},
		{"catalogs", "owner_id", "INTEGER REFERENCES users(id) ON DELETE SET NULL"},
		{"catalogs", "private", "INTEGER NOT NULL DEFAULT 0"},
		{"deep_reads", "checks", "INTEGER NOT NULL DEFAULT 1"},
		{"deep_reads", "held", "INTEGER NOT NULL DEFAULT 0"},
		{"deep_reads", "proposed_level", "INTEGER"},
		{"deep_reads", "proposal", "TEXT NOT NULL DEFAULT ''"},
		{"token_usage", "cost", "REAL"},
		{"books", "content_version", "INTEGER NOT NULL DEFAULT 0"},
		{"books", "content_amounts", "TEXT NOT NULL DEFAULT ''"},
		{"books", "premise", "TEXT NOT NULL DEFAULT ''"},
		{"catalogs", "physical", "INTEGER NOT NULL DEFAULT 0"},
	} {
		if err := addColumn(d, c[0], c[1], c[2]); err != nil {
			return err
		}
	}
	return moveLGBTQ(d)
}

// moveLGBTQ turns the LGBTQ+ flag and hide rule (before v1.19) into the
// content item "lgbtq". The old columns are cleared, so it runs only once.
func moveLGBTQ(d *sqlx.DB) error {
	tx, err := d.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`INSERT OR IGNORE INTO book_content (book_id, key, source)
			SELECT id, 'lgbtq', CASE WHEN analysis_model LIKE 'manual:%' THEN 'parent'
				WHEN analysis_model LIKE 'deep:%' THEN 'deep' ELSE 'ai' END
			FROM books WHERE lgbtq_content = 1`,
		`UPDATE books SET lgbtq_content = 0 WHERE lgbtq_content = 1`,
		`INSERT OR IGNORE INTO user_hidden_content (user_id, key) SELECT id, 'lgbtq' FROM users WHERE hide_lgbtq = 1`,
		`UPDATE users SET hide_lgbtq = 0 WHERE hide_lgbtq = 1`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("move LGBTQ+ to content items: %w", err)
		}
	}
	return tx.Commit()
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
