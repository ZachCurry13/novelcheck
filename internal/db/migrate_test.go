package db

import (
	"path/filepath"
	"testing"

	"github.com/jmoiron/sqlx"
)

// A v1.0 database (role CHECK without 'editor') must upgrade in place without
// losing users or cascading deletes into sessions and queues.
func TestAddEditorRoleMigration(t *testing.T) {
	dir := t.TempDir()
	old, err := sqlx.Open("sqlite", filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	v10 := `
CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL UNIQUE COLLATE NOCASE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'restricted' CHECK (role IN ('admin', 'restricted')),
  hide_open_door INTEGER NOT NULL DEFAULT 0, hide_nudity INTEGER NOT NULL DEFAULT 0,
  hide_solo_acts INTEGER NOT NULL DEFAULT 0, hide_innuendo INTEGER NOT NULL DEFAULT 0,
  hide_dark_occult INTEGER NOT NULL DEFAULT 0, hide_lgbtq INTEGER NOT NULL DEFAULT 0,
  hide_unrated INTEGER NOT NULL DEFAULT 0,
  delivery_method TEXT NOT NULL DEFAULT 'none' CHECK (delivery_method IN ('none', 'email', 'koreader')),
  kindle_email TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE sessions (token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at DATETIME NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
INSERT INTO users (username, password_hash, role) VALUES ('admin', 'x', 'admin');
INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ('t', 1, datetime('now', '+1 day'));`
	if _, err := old.Exec(v10); err != nil {
		t.Fatal(err)
	}
	old.Close()

	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('wife', 'x', 'editor')`); err != nil {
		t.Fatalf("editor role still rejected after migration: %v", err)
	}
	var n int
	if err := d.Get(&n, `SELECT COUNT(*) FROM sessions`); err != nil || n != 1 {
		t.Fatalf("sessions lost during migration: %d %v", n, err)
	}
	var fk int
	if err := d.Get(&fk, `PRAGMA foreign_keys`); err != nil || fk != 1 {
		t.Fatalf("foreign keys not re-enabled: %d %v", fk, err)
	}
	// Re-opening is a no-op.
	d.Close()
	if d, err = Open(dir); err != nil {
		t.Fatal(err)
	}
}

// The LGBTQ+ flag and hide rule (before v1.19) become the content item
// "lgbtq", once: a parent can later remove it without it coming back.
func TestMoveLGBTQ(t *testing.T) {
	d, err := OpenDSN("file:movelgbtq?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.MustExec(`INSERT INTO users (id, username, password_hash, role, hide_lgbtq) VALUES (1, 'kid', 'x', 'restricted', 1)`)
	d.MustExec(`INSERT INTO books (id, norm_key, title, lgbtq_content, analysis_model) VALUES (1, 'a', 'A', 1, 'manual: mom'), (2, 'b', 'B', 0, 'gpt')`)
	if err := migrate(d); err != nil {
		t.Fatal(err)
	}
	var src string
	if err := d.Get(&src, `SELECT source FROM book_content WHERE book_id = 1 AND key = 'lgbtq'`); err != nil || src != "parent" {
		t.Fatalf("book item: %q %v", src, err)
	}
	var rules, left int
	_ = d.Get(&rules, `SELECT COUNT(*) FROM user_hidden_content WHERE user_id = 1 AND key = 'lgbtq'`)
	_ = d.Get(&left, `SELECT (SELECT COUNT(*) FROM books WHERE lgbtq_content = 1) + (SELECT COUNT(*) FROM users WHERE hide_lgbtq = 1)`)
	if rules != 1 || left != 0 {
		t.Fatalf("rules %d, old marks left %d", rules, left)
	}
	d.MustExec(`DELETE FROM book_content`)
	if err := migrate(d); err != nil {
		t.Fatal(err)
	}
	var n int
	if _ = d.Get(&n, `SELECT COUNT(*) FROM book_content`); n != 0 {
		t.Fatal("the move ran twice")
	}
}
