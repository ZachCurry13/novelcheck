package hardcover

import (
	"context"
	"log"
	"time"

	"github.com/zachcurry13/novelcheck/internal/safe"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// SyncUser pulls one person's Hardcover shelves into NovelCheck and says
// how many of their books are in the family's library.
func SyncUser(ctx context.Context, st *store.Store, c *Client, userID int64) (books, matched int, err error) {
	shelves, err := c.Shelves(ctx)
	if err != nil {
		return 0, 0, err
	}
	in := make([]store.HardcoverBook, 0, len(shelves))
	for _, b := range shelves {
		in = append(in, store.HardcoverBook{Title: b.Title, Authors: b.Authors, ISBNs: b.ISBNs, Status: b.Status,
			Percent: b.Percent, UpdatedAt: b.UpdatedAt.Unix()})
	}
	matched, err = st.SaveHardcover(userID, in)
	return len(in), matched, err
}

// Loop syncs everyone with a token an hour after start and then hourly.
func Loop(ctx context.Context, st *store.Store) {
	wait := time.Minute
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = time.Hour
		for _, id := range st.HardcoverUsers() {
			id := id
			_ = safe.Run("hardcover sync", func() error {
				cctx, cancel := context.WithTimeout(ctx, time.Minute)
				defer cancel()
				c := &Client{Token: st.HardcoverToken(id)}
				if _, _, err := SyncUser(cctx, st, c, id); err != nil {
					log.Printf("hardcover sync for user %d: %v", id, err)
				}
				if st.HardcoverPush(id) {
					if _, err := PushUser(cctx, st, c, id); err != nil {
						log.Printf("hardcover push for user %d: %v", id, err)
					}
				}
				return nil
			})
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second): // far under Hardcover's limits
			}
		}
	}
}

// PushUser sends a person's reading state to their Hardcover (up to 15
// books a run): reading or finished books on your Nightstand, matched by ISBN.
// Books Hardcover doesn't know are marked done so they aren't retried every hour.
func PushUser(ctx context.Context, st *store.Store, c *Client, userID int64) (sent int, err error) {
	for _, p := range st.HardcoverPending(userID, 15) {
		ed, ok, err := c.FindEdition(ctx, p.ISBN)
		if err != nil {
			return sent, err
		}
		if !ok {
			st.MarkHardcoverPushed(userID, p.BookID, p.Status, p.Percent)
			continue
		}
		pages := 0
		if p.Status == Read {
			pages = ed.Pages
		} else if ed.Pages > 0 {
			pages = int(p.Percent * float64(ed.Pages))
		}
		if err := c.Push(ctx, ed, p.Status, pages, time.Now()); err != nil {
			return sent, err
		}
		st.MarkHardcoverPushed(userID, p.BookID, p.Status, p.Percent)
		sent++
		select {
		case <-ctx.Done():
			return sent, ctx.Err()
		case <-time.After(1200 * time.Millisecond): // far under Hardcover's limits
		}
	}
	return sent, nil
}
