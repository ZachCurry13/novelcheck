package main

import (
	"context"
	"fmt"
	"log"

	"github.com/zachcurry13/novelcheck/internal/aitools"
	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/calibre"
	"github.com/zachcurry13/novelcheck/internal/collections"
	"github.com/zachcurry13/novelcheck/internal/deepread"
	"github.com/zachcurry13/novelcheck/internal/discover"
	"github.com/zachcurry13/novelcheck/internal/events"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// background is the work NovelCheck does on its own. Safe mode keeps it off
// until an admin leaves safe mode; the web app runs either way.
type background struct {
	st          *store.Store
	worker      *analyzer.Worker
	collections *collections.Service
	aiTools     *aitools.Service
	syncer      *calibre.Syncer
	deep        *deepread.Runner
	discover    *discover.Service
}

func (b *background) start(ctx context.Context) {
	// Titles stored before tidying ("01 - Dune") get their plain title and series number.
	if n, err := b.st.TidyTitles(); err == nil && n > 0 {
		log.Printf("tidied %d titles with track or series numbers", n)
	}
	go b.worker.Run(ctx)
	// Ratings from Deep Scans made before the stricter checks could come from
	// one misread part: they go back to a rating from the book's description.
	if old := b.st.OldDeepRatings(); len(old) > 0 {
		b.worker.Enqueue(false, old...)
		b.st.Notify("info", "deep-scan", fmt.Sprintf("Deep Scan got stricter. %d book(s) it rated before are being rated from their description again; "+
			"you can Deep Scan them again from the Deep Scan page.", len(old)), "#/deepscan")
	}
	go b.collections.Loop(ctx)
	go events.Loop(ctx, b.st) // events are archived when they end
	go b.aiTools.Loop(ctx)    // daily: newer versions of the Ollama models in use
	go b.syncer.Loop(ctx)
	go b.deep.Run(ctx)      // Deep Scans: full-text reading of chosen books, one at a time
	go b.discover.Loop(ctx) // outside book lists refreshed daily; their books rated a few dozen a day
	log.Printf("background work started")
}
