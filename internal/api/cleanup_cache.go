package api

import (
	"fmt"
	"sync"
	"time"
)

// The Library cleanup summary takes a second or two to work out (it compares every book with every other
// for the "listed twice" pairs). It only changes when the library does, so the last answer is kept in
// memory with a fingerprint of the library, and given again while the fingerprint is the same (for ten minutes at most).
type cleanupCache struct {
	mu    sync.Mutex
	print string
	body  map[string]any
	at    time.Time
}

// libraryPrint is cheap to read and changes whenever something the summary counts changes: a sync, a join,
// a removal, a title tidied, a pair set aside, a conversion that failed.
func (s *Server) libraryPrint() string {
	var p struct {
		Books  int64  `db:"books"`
		MaxB   int64  `db:"maxb"`
		Copies int64  `db:"copies"`
		Fixes  int64  `db:"fixes"`
		Ign    int64  `db:"ign"`
		Fail   int64  `db:"fail"`
		Mod    string `db:"mod"`
	}
	_ = s.Store.DB.Get(&p, `SELECT (SELECT COUNT(*) FROM books) AS books, (SELECT COALESCE(MAX(id), 0) FROM books) AS maxb,
		(SELECT COUNT(*) FROM catalog_books) AS copies, (SELECT COUNT(*) FROM books WHERE title_fix != '') AS fixes,
		(SELECT COUNT(*) FROM dup_ignores) AS ign, (SELECT COUNT(*) + 1000 * COALESCE(SUM(as_is), 0) FROM convert_failures) AS fail,
		(SELECT COALESCE(MAX(modified), '') FROM catalog_books) AS mod`)
	return fmt.Sprintf("%d|%d|%d|%d|%d|%d|%s|%s|%s", p.Books, p.MaxB, p.Copies, p.Fixes, p.Ign, p.Fail, p.Mod,
		s.Store.Setting("calibre_last_sync"), s.Store.Setting("calibre_server_url"))
}

// get returns the kept summary when the library has not changed since.
func (c *cleanupCache) get(print string) (map[string]any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Ten minutes at most: a title edited by hand changes no count, and should still show up soon.
	if c.body == nil || c.print != print || time.Since(c.at) > 10*time.Minute {
		return nil, false
	}
	out := make(map[string]any, len(c.body))
	for k, v := range c.body {
		out[k] = v
	}
	return out, true
}

func (c *cleanupCache) put(print string, body map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.print, c.body, c.at = print, body, time.Now()
}
