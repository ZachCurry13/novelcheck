package api

import (
	"context"
	"log"
	"time"

	"github.com/zachcurry13/novelcheck/internal/kosync"
)

// Every file in the Calibre library is fingerprinted the way KOReader does it,
// slowly in the background, so a book copied to an e-reader is recognised by
// its file, whatever the file is called there. Books are matched by title as
// well, but a fingerprint is exact.

// IndexFingerprintsOnce fingerprints up to limit files not done yet and says
// how many it hashed.
func (s *Server) IndexFingerprintsOnce(limit int) int {
	n := 0
	for _, f := range s.Store.UnhashedFiles(limit) {
		if !s.insideCalibre(f.Path) {
			s.Store.MarkFingerprint(f.Path, "", f.BookID)
			continue
		}
		sum, err := kosync.PartialMD5(f.Path)
		if err != nil {
			sum = "" // missing or unreadable: don't try it again every round
		} else {
			n++
		}
		s.Store.MarkFingerprint(f.Path, sum, f.BookID)
	}
	return n
}

// IndexFingerprints keeps the index up to date until ctx ends.
func (s *Server) IndexFingerprints(ctx context.Context) {
	wait := 30 * time.Second
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = 30 * time.Minute
		total := 0
		for {
			n := s.IndexFingerprintsOnce(200)
			total += n
			if n == 0 || ctx.Err() != nil {
				break
			}
			time.Sleep(500 * time.Millisecond) // be gentle with the disk
		}
		if total > 0 {
			log.Printf("fingerprinted %d library files", total)
		}
	}
}
