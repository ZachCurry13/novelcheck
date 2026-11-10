// Package db opens the NovelCheck SQLite database and applies the schema.
package db

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite" // pure-Go driver, registered as "sqlite"
)

//go:embed schema.sql
var schema string

// FileName is the name of the application database inside the data dir.
const FileName = "novelcheck.db"

// Open creates the data directory if needed, opens novelcheck.db with WAL
// journaling and foreign keys enabled, and applies the schema.
func Open(dataDir string) (*sqlx.DB, error) {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	path := filepath.Join(dataDir, FileName)
	// busy_timeout: a write waits up to 30 s for another to finish (as it did when there was one connection);
	// _txlock=immediate: a transaction takes its place in the write queue at its start, so it can't fail halfway.
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(30000)&_txlock=immediate"
	d, err := OpenDSN(dsn)
	if err != nil {
		return nil, err
	}
	// WAL lets reads go on while a write happens, so a few connections stop one slow request from holding up every
	// page (with one connection each request waited for the one before it). NOVELCHECK_DB_CONNS=1 goes back to one.
	conns := 4
	if n, err := strconv.Atoi(os.Getenv("NOVELCHECK_DB_CONNS")); err == nil && n >= 1 && n <= 16 {
		conns = n
	}
	d.SetMaxOpenConns(conns)
	return d, nil
}

// OpenDSN opens an arbitrary SQLite DSN and applies the schema (used by tests).
func OpenDSN(dsn string) (*sqlx.DB, error) {
	d, err := sqlx.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite allows a single writer; one connection avoids SQLITE_BUSY churn.
	d.SetMaxOpenConns(1)
	if _, err := d.Exec(schema); err != nil {
		d.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := migrate(d); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}
