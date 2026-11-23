package api

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/safe"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The last Check a book runs with how long each part took and every AI asked, for the bug log: "Check a book
// takes too long" can then be answered from the log.

type checkRun struct {
	At     time.Time
	User   string
	What   string // "photo" or what was typed
	Title  string
	Lookup time.Duration
	Rating time.Duration // 0 when the book was rated already
	Steps  string        // the AIs asked (analyzer.Trace)
}

var checkRuns struct {
	mu   sync.Mutex
	list []checkRun // newest last, at most 20
}

func noteCheckRun(c checkRun) {
	checkRuns.mu.Lock()
	defer checkRuns.mu.Unlock()
	checkRuns.list = append(checkRuns.list, c)
	if len(checkRuns.list) > 20 {
		checkRuns.list = checkRuns.list[len(checkRuns.list)-20:]
	}
}

// writeCheckRuns adds the bug log's Check a book section, newest first.
func writeCheckRuns(b *strings.Builder) {
	checkRuns.mu.Lock()
	defer checkRuns.mu.Unlock()
	fmt.Fprintf(b, "\n== Check a book (since the server started, newest first, %d) ==\n", len(checkRuns.list))
	for i := len(checkRuns.list) - 1; i >= 0; i-- {
		c := checkRuns.list[i]
		rating := "rated already"
		if c.Rating > 0 {
			rating = fmt.Sprintf("rating %.1fs", c.Rating.Seconds())
		}
		fmt.Fprintf(b, "%s UTC · %s · %s → %q · lookup %.1fs · %s\n", c.At.UTC().Format("2006-01-02 15:04:05"), c.User, c.What, c.Title, c.Lookup.Seconds(), rating)
		if c.Steps != "" {
			fmt.Fprintf(b, "    AIs: %s\n", c.Steps)
		}
	}
}

// rateChecked rates a checked book now, as rateSoon does, with a person waiting (llm.Quick), and notes the run.
func (s *Server) rateChecked(b *store.Book, run checkRun) {
	if b.Status == "analyzed" || b.Status == "processing" {
		noteCheckRun(run)
		return
	}
	id, title := b.ID, b.Title
	safe.Go("rate a checked book", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		ctx, trace := analyzer.WithTrace(llm.Quick(ctx))
		start := time.Now()
		if err := s.Worker.RateNow(ctx, id); err != nil {
			log.Printf("rating %q failed: %v", title, err)
		}
		run.Rating = max(time.Since(start), time.Millisecond)
		if run.Steps != "" {
			run.Steps += " · "
		}
		run.Steps += trace.String()
		noteCheckRun(run)
	})
}
