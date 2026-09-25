// Package genrefill asks the AI for the genres of library books Calibre has
// no tags for, 25 books per call, when an admin starts it. It stays within
// the hourly token cap and can be stopped at any time.
package genrefill

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/safe"
	"github.com/zachcurry13/novelcheck/internal/store"
)

const (
	batch       = 25
	tokensIn    = 90  // per book: title, author and a short description
	tokensOut   = 20  // per book in the answer
	tokensFixed = 450 // the instructions, per call
)

type Filler struct {
	Store   *store.Store
	mu      sync.Mutex
	running bool
	done    int
	total   int
	lastErr string
	cancel  context.CancelFunc
}

// Status is what the Admin page shows.
type Status struct {
	Missing int     `json:"missing"`  // library books with no genre
	Running bool    `json:"running"`  // filling in now
	Done    int     `json:"done"`     // books done in this run
	Total   int     `json:"total"`    // books this run started with
	EstCost float64 `json:"est_cost"` // USD for the missing ones with the main AI (0 for Ollama)
	Error   string  `json:"error,omitempty"`
}

func (f *Filler) Status() Status {
	missing := f.Store.CountMissingGenres()
	f.mu.Lock()
	defer f.mu.Unlock()
	st := Status{Missing: missing, Running: f.running, Done: f.done, Total: f.total, Error: f.lastErr}
	if ais := f.Store.AIConfigs(); len(ais) > 0 {
		calls := (missing + batch - 1) / batch
		in, out := missing*tokensIn+calls*tokensFixed, missing*tokensOut
		st.EstCost = float64(in)*ais[0].PriceIn/1e6 + float64(out)*ais[0].PriceOut/1e6
	}
	return st
}

// Start begins filling in genres in the background.
func (f *Filler) Start() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.running {
		return fmt.Errorf("already filling in genres")
	}
	ctx, cancel := context.WithCancel(context.Background())
	f.running, f.done, f.total, f.lastErr, f.cancel = true, 0, f.Store.CountMissingGenres(), "", cancel
	safe.Go("filling in genres", func() { f.run(ctx) })
	return nil
}

// Stop ends a run after the current call.
func (f *Filler) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cancel != nil {
		f.cancel()
	}
}

func (f *Filler) run(ctx context.Context) {
	defer func() {
		f.mu.Lock()
		f.running, f.cancel = false, nil
		done, errMsg := f.done, f.lastErr
		f.mu.Unlock()
		switch {
		case errMsg != "":
			f.Store.Notify("warning", "genres", "Filling in genres stopped: "+errMsg, "#/admin")
		case done > 0:
			f.Store.NotifyRoutine("genres", fmt.Sprintf("Genres filled in for %d books.", done), "#/library")
		}
	}()
	system := llm.GenreSystem()
	for ctx.Err() == nil {
		books, err := f.Store.MissingGenres(batch)
		if err != nil || len(books) == 0 {
			f.fail(err)
			return
		}
		if !f.waitForBudget(ctx, len(books)) {
			return
		}
		ids := make(map[int64]bool, len(books))
		list := make([]llm.GenreBook, len(books))
		for i, b := range books {
			ids[b.ID] = true
			list[i] = llm.GenreBook{ID: b.ID, Title: b.Title, Author: b.Author, Description: b.Description}
		}
		var answers map[int64]llm.GenreAnswer
		_, err = llm.Ask(ctx, f.Store, f.Store.AIConfigs(), 0, system, llm.GenreUser(list), func(out string) (err error) {
			answers, err = llm.ParseGenres(out, ids)
			return err
		})
		if err != nil {
			if ctx.Err() == nil {
				f.fail(err)
			}
			return
		}
		// Books the answer skipped are marked done too, so a run always ends.
		for _, b := range books {
			a := answers[b.ID]
			if err := f.Store.SetAIGenres(b.ID, a.Keys, a.Kind); err != nil {
				f.fail(err)
				return
			}
		}
		f.mu.Lock()
		f.done += len(books)
		f.mu.Unlock()
	}
}

func (f *Filler) fail(err error) {
	if err == nil {
		return
	}
	f.mu.Lock()
	f.lastErr = err.Error()
	f.mu.Unlock()
}

// waitForBudget pauses while the next call would pass the hourly token cap.
func (f *Filler) waitForBudget(ctx context.Context, n int) bool {
	est := n*(tokensIn+tokensOut) + tokensFixed
	for {
		limit := f.Store.SettingInt(store.KeyTokensPerHour)
		used, err := f.Store.TokensSince("-1 hour")
		if limit <= 0 || err != nil || used+est <= limit || used == 0 {
			return true
		}
		secs, _ := f.Store.SecondsUntilWindowFrees()
		select {
		case <-ctx.Done():
			return false
		case <-time.After(time.Duration(max(secs, 15)) * time.Second):
		}
	}
}
