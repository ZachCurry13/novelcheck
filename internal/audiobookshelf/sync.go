package audiobookshelf

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Result says what a sync did.
type Result struct {
	Libraries int `json:"libraries"`
	Books     int `json:"books"`
	Removed   int `json:"removed"`
}

var running sync.Mutex

// FromSettings is the shared connection the admin set up; ok is false until both parts are filled in.
func FromSettings(st *store.Store) (*Client, bool) {
	base := CleanBase(st.Setting(store.KeyABSURL))
	tok := st.Setting(store.KeyABSToken)
	return &Client{Base: base, Token: tok}, base != "" && tok != ""
}

// Sync reads every book library and saves its audiobooks into the Audiobookshelf
// catalog. Audiobooks that are gone from the server are dropped only when the
// whole read worked, so an outage never empties the library.
func Sync(ctx context.Context, st *store.Store, c *Client) (Result, error) {
	var res Result
	if !running.TryLock() {
		return res, fmt.Errorf("a sync is already running")
	}
	defer running.Unlock()
	libs, err := c.Libraries(ctx)
	if err != nil {
		return res, err
	}
	cat, err := st.EnsureCatalog(store.AudiobookshelfCatalog, "custom")
	if err != nil {
		return res, err
	}
	keep := map[string]bool{}
	for _, l := range libs {
		if l.MediaType != "book" {
			continue
		}
		res.Libraries++
		for page := 0; ; page++ {
			items, total, err := c.Items(ctx, l.ID, page, 100)
			if err != nil {
				return res, fmt.Errorf("reading %q: %w", l.Name, err)
			}
			for _, it := range items {
				if _, err := st.SaveAudioItem(cat, it); err != nil {
					return res, err
				}
				keep[it.ID] = true
				res.Books++
			}
			if (page+1)*100 >= total {
				break
			}
		}
	}
	res.Removed, err = st.PruneCatalog(cat, keep)
	if err == nil {
		st.TidySame()
	}
	return res, err
}

// Record keeps the outcome for the admin page.
func Record(st *store.Store, res Result, err error) {
	_ = st.SetSetting(store.KeyABSLastSync, time.Now().UTC().Format(time.RFC3339))
	msg := fmt.Sprintf("%d audiobooks from %d libraries", res.Books, res.Libraries)
	if res.Removed > 0 {
		msg += fmt.Sprintf(", %d no longer on the server", res.Removed)
	}
	if err != nil {
		msg = "failed: " + err.Error()
	}
	_ = st.SetSetting(store.KeyABSLastResult, msg)
}

// Loop reads the server again every few hours (the admin sets how often; 0 = only on request).
func Loop(ctx context.Context, st *store.Store) {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for {
		if c, ok := FromSettings(st); ok {
			if h := st.SettingInt(store.KeyABSPollHours); h > 0 {
				last, _ := time.Parse(time.RFC3339, st.Setting(store.KeyABSLastSync))
				if time.Since(last) >= time.Duration(h)*time.Hour {
					res, err := Sync(ctx, st, c)
					Record(st, res, err)
					if err != nil {
						log.Printf("audiobookshelf sync: %v", err)
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Busy reports whether a sync is running right now.
func Busy() bool {
	if running.TryLock() {
		running.Unlock()
		return false
	}
	return true
}
