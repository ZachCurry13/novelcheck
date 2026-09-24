package delivery

import (
	"os"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// formatRank orders formats by Send-to-Kindle compatibility (EPUB preferred;
// Amazon no longer accepts MOBI/AZW3 by email).
var formatRank = map[string]int{"epub": 0, "pdf": 1, "azw3": 2, "mobi": 3}

// BestFile picks the most deliverable copy that exists on this server's disk.
// Copies from browser drive scans have no server-side file and are skipped.
func BestFile(copies []store.BookCopy, emailOnly bool) (store.BookCopy, bool) {
	var best store.BookCopy
	bestRank := 99
	for _, c := range copies {
		if c.Source != "calibre" || c.Path == "" {
			continue
		}
		r, ok := formatRank[strings.ToLower(c.Format)]
		if !ok {
			r = 10
		}
		if emailOnly && r > 1 {
			continue
		}
		if _, err := os.Stat(c.Path); err != nil {
			continue
		}
		if r < bestRank {
			best, bestRank = c, r
		}
	}
	return best, bestRank < 99
}
