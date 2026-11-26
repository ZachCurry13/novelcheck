package api

import (
	"context"
	"log"
	"time"

	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/llm"
)

// A rating the small model "wasn't sure" of (the Not sure chip) gets a second opinion from the bigger model on the
// Deep Scan machine, by itself: a few books every few minutes, only while that machine is on, inside the hours
// automatic rating is allowed (the same quiet hours as the rest of the background AI work), and only when that
// machine is on the home network (so it costs nothing). Each book gets one second opinion per rules version: if
// the big model is unsure too, the Not sure chip stays for a parent to look at.

const bigRecheckBatch = 3

// BigRecheckLoop looks for "not sure" ratings every five minutes: the automatic way (when automatic rating is on) and the
// nightly way (an admin chose "this many a night" on the AI page).
func (s *Server) BigRecheckLoop(ctx context.Context) {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n := s.RecheckNotSure(ctx, bigRecheckBatch)
			n += s.recheckTonight(ctx)
			if n > 0 {
				log.Printf("bigger model: %d unsure ratings looked at again", n)
			}
		}
	}
}

// RecheckNotSure rates up to limit "not sure" books again on the bigger model and says how many it did (the automatic way).
func (s *Server) RecheckNotSure(ctx context.Context, limit int) int {
	if !analyzer.AutoRateOn(s.Store) || !analyzer.RateHoursOpen(s.Store, time.Now()) {
		return 0
	}
	return s.recheck(ctx, limit)
}

// recheck is the work itself: up to limit unsure books, on the bigger model when it is a machine on the home network and is on.
func (s *Server) recheck(ctx context.Context, limit int) int {
	ai, ok := s.Store.DeepAI()
	if !ok || s.Deep == nil || !llm.IsLocal(ai.BaseURL) || s.Deep.MachineAway() {
		return 0
	}
	ids, err := s.Store.NotSureToRecheck(limit)
	if err != nil {
		return 0
	}
	done := 0
	for _, id := range ids {
		if ctx.Err() != nil || s.Deep.MachineAway() {
			break
		}
		rctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		err := s.Worker.RateWith(rctx, id, ai)
		cancel()
		if err != nil && s.Deep.MachineAway() { // it went off meanwhile: try again when it is back
			break
		}
		s.Store.MarkBigChecked(id) // one second opinion per rules version, even when it could not be given
		bigRun.step()
		done++
	}
	return done
}
