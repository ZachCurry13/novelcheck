package api

import (
	"time"

	"github.com/zachcurry13/novelcheck/internal/hardcover"
)

// SetHardcoverURL points Hardcover calls at a fake server and removes the wait between calls (tests only).
func SetHardcoverURL(u string) {
	hardcoverURL = u
	if u != "" {
		hardcover.MinGap = 0
	} else {
		hardcover.MinGap = 1100 * time.Millisecond
	}
}
