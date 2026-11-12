package api

import (
	"os"
	"sync"
	"time"
)

// Calibre's recycle bin can hold hundreds of books on a network share, and reading them all takes minutes:
// longer than the remote-access tunnel waits for an answer. So the bin is read in the background and the
// result kept in memory. It is read again when the bin folder changes (a book removed or put back), when it
// is over an hour old, or when the page asks for a fresh read.
type trashCache struct {
	mu          sync.Mutex
	books       []trashBook
	found       bool
	stamp       string // the bin folder's change time and size when it was read
	at          time.Time
	reading     bool
	done, total int
}

func (s *Server) trashStamp() string {
	dir := s.trashDir()
	if dir == "" {
		return ""
	}
	info, err := os.Stat(dir)
	if err != nil {
		return "none"
	}
	return info.ModTime().UTC().Format(time.RFC3339Nano)
}

// trashListing answers the bin page: the books when a current reading is kept, otherwise how far the
// background reading has got (starting it when none runs).
func (s *Server) trashListing(fresh bool) map[string]any {
	c := &s.trash
	stamp := s.trashStamp()
	c.mu.Lock()
	defer c.mu.Unlock()
	current := !c.at.IsZero() && c.stamp == stamp && time.Since(c.at) < time.Hour && !fresh
	if current && !c.reading {
		// Checked against the library on every answer (cheap, and the library changes as books are put back).
		return map[string]any{"found": c.found, "books": s.matchTrash(c.books), "count": len(c.books), "read_at": c.at.UTC().Format(time.RFC3339),
			"removals": s.Store.CalibreRemovals(1000)} // who removed what through NovelCheck
	}
	if !c.reading {
		c.reading, c.done, c.total = true, 0, 0
		go func() {
			books, found := s.readTrash(func(done, total int) {
				c.mu.Lock()
				c.done, c.total = done, total
				c.mu.Unlock()
			})
			c.mu.Lock()
			c.books, c.found, c.stamp, c.at, c.reading = books, found, stamp, time.Now(), false
			c.mu.Unlock()
		}()
	}
	return map[string]any{"reading": true, "done": c.done, "total": c.total}
}

// forgetTrash drops the kept reading (after a book is put back).
func (s *Server) forgetTrash() {
	s.trash.mu.Lock()
	s.trash.at = time.Time{}
	s.trash.mu.Unlock()
}
