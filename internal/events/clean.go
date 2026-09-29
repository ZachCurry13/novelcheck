package events

import (
	"context"
	"log"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Loop removes unpinned events older than 30 days, every few hours.
func Loop(ctx context.Context, st *store.Store) {
	t := time.NewTicker(6 * time.Hour)
	defer t.Stop()
	for {
		if n, err := st.CleanEvents(); err != nil {
			log.Printf("events cleanup: %v", err)
		} else if n > 0 {
			log.Printf("events cleanup: removed %d old event(s)", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
