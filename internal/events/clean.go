package events

import (
	"context"
	"log"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Loop archives events whose end passed (and unpinned ones without an end
// after 30 days), every few minutes so an event leaves soon after it ends.
func Loop(ctx context.Context, st *store.Store) {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for {
		if n, err := st.ArchiveEnded(); err != nil {
			log.Printf("events archive: %v", err)
		} else if n > 0 {
			log.Printf("events archive: archived %d event(s)", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
