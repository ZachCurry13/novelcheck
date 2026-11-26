package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/llm"
)

// "Not sure" ratings and the bigger model (AI & Scans): an admin sees every rating the small AI was unsure of and decides: all of them
// in one batch now, or this many a night (inside the hours automatic rating is allowed), or none. One second opinion per rules version.

const keyBigNightly = "bigrecheck_nightly" // books a night; 0 or empty = only the automatic way

// bigRun is the batch an admin started with "Look at them now".
var bigRun = &runState{}

type runState struct {
	mu          sync.Mutex
	on          bool
	done, total int
}

func (r *runState) step() {
	r.mu.Lock()
	if r.on {
		r.done++
	}
	r.mu.Unlock()
}

func (r *runState) snapshot() map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return map[string]any{"running": r.on, "done": r.done, "total": r.total}
}

// nightKey names the night a time belongs to (a night that crosses midnight is one night): the date twelve hours earlier, in the
// admin's time zone.
func (s *Server) nightKey(now time.Time) string {
	loc := time.UTC
	if tz := s.Store.Setting("auto_rate_tz"); tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	return now.In(loc).Add(-12 * time.Hour).Format("2006-01-02")
}

// doneTonight is how many the nightly way did in this night so far.
func (s *Server) doneTonight(now time.Time) int {
	key, count, _ := strings.Cut(s.Store.Setting("bigrecheck_night"), ":")
	if key != s.nightKey(now) {
		return 0
	}
	n, _ := strconv.Atoi(count)
	return n
}

// recheckTonight looks at a few books if a nightly number is set, the hours are open and tonight's number is not used up.
func (s *Server) recheckTonight(ctx context.Context) int {
	nightly := s.Store.SettingInt(keyBigNightly)
	now := time.Now()
	if nightly <= 0 || !analyzer.RateHoursOpen(s.Store, now) {
		return 0
	}
	left := nightly - s.doneTonight(now)
	if left <= 0 {
		return 0
	}
	n := s.recheck(ctx, min(bigRecheckBatch, left))
	if n > 0 {
		_ = s.Store.SetSetting("bigrecheck_night", s.nightKey(now)+":"+strconv.Itoa(s.doneTonight(now)+n))
	}
	return n
}

// handleNotSure: GET lists the unsure ratings and the choices; PUT {"nightly": n} sets the nightly number.
func (s *Server) handleNotSure(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		var body struct {
			Nightly int `json:"nightly"`
		}
		if !readJSON(w, r, &body, 1<<10) {
			return
		}
		if body.Nightly < 0 || body.Nightly > 1000 {
			writeErr(w, http.StatusBadRequest, "a number from 0 to 1000")
			return
		}
		if err := s.Store.SetSetting(keyBigNightly, strconv.Itoa(body.Nightly)); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	books := s.Store.NotSureList()
	waiting := 0
	for _, b := range books {
		if !b.Checked {
			waiting++
		}
	}
	ai, ok := s.Store.DeepAI()
	writeJSON(w, http.StatusOK, map[string]any{"books": books, "waiting": waiting, "nightly": s.Store.SettingInt(keyBigNightly),
		"done_tonight": s.doneTonight(time.Now()), "machine": ok, "local": ok && llm.IsLocal(ai.BaseURL),
		"away": ok && s.Deep != nil && s.Deep.MachineAway(), "run": bigRun.snapshot()})
}

// handleNotSureRun: POST {"limit": n} looks at n of the waiting ratings now (0 = all of them), in the background.
func (s *Server) handleNotSureRun(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Limit int `json:"limit"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	ai, ok := s.Store.DeepAI()
	if !ok || s.Deep == nil || !llm.IsLocal(ai.BaseURL) {
		writeErr(w, http.StatusConflict, "the bigger model needs the Deep Scan machine on your home network (AI & Scans)")
		return
	}
	if s.Deep.MachineAway() {
		writeErr(w, http.StatusConflict, "the Deep Scan machine isn't answering right now")
		return
	}
	bigRun.mu.Lock()
	if bigRun.on {
		bigRun.mu.Unlock()
		writeErr(w, http.StatusConflict, "a batch is already running")
		return
	}
	waiting, _ := s.Store.NotSureToRecheck(100000)
	total := len(waiting)
	if body.Limit > 0 && body.Limit < total {
		total = body.Limit
	}
	bigRun.on, bigRun.done, bigRun.total = true, 0, total
	bigRun.mu.Unlock()
	go func() {
		defer func() { bigRun.mu.Lock(); bigRun.on = false; bigRun.mu.Unlock() }()
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Hour)
		defer cancel()
		left := total
		for left > 0 {
			n := s.recheck(ctx, min(5, left))
			if n == 0 {
				return
			}
			left -= n
		}
	}()
	writeJSON(w, http.StatusAccepted, bigRun.snapshot())
}
