package api

import (
	"sync"
	"time"
)

// The slow part of the Admin status: what needs re-rating (every book against the current rules), the language
// check of every summary, and the spending totals. It changes slowly, so it is kept in memory and worked out
// again in the background when it is a minute old (the warm loop does it too), instead of on every look.

type statusSlow struct {
	out     map[string]any
	perBook float64
	at      time.Time
	ver     uint64 // the change counter when it was worked out
}

type slowStatusCache struct {
	mu   sync.Mutex
	v    *statusSlow
	busy bool
}

const slowStatusEvery = 45 * time.Second

func (s *Server) computeSlowStatus() *statusSlow {
	ver := s.rcache.ver.Load()
	usage, _ := s.Store.Usage()
	perBook := usage.AvgPerBook
	if perBook == 0 {
		perBook = defaultTokensPerBook
	}
	rerate, _ := s.Store.RerateCandidates()
	rerateLib, _ := s.Store.RerateCandidatesLibrary()
	aiRated, _ := s.Store.AIRatedIDs()
	return &statusSlow{at: time.Now(), ver: ver, perBook: perBook, out: map[string]any{
		"rerate_candidates": len(rerate),
		"rerate_library":    len(rerateLib), // the same, without books only on Discover lists and events
		"ai_rated":          len(aiRated),
		"changed_books":     s.Store.ChangedSinceRated(),
		"non_english":       s.nonEnglishCount(),
		"usage":             usage,
		"cost_spent":        s.Store.SpentUSD(),
		"cost_spent_30d":    s.Store.SpentSinceUSD("-30 days"),
		"cost_power":        s.Store.ElectricityUSD(),
	}}
}

// slowStatus is the kept slow part: worked out now only the first time; a minute-old one is given at once and
// worked out again behind it.
func (s *Server) slowStatus() *statusSlow {
	c := &s.slowStat
	c.mu.Lock()
	v := c.v
	if v != nil && v.ver == s.rcache.ver.Load() && (time.Since(v.at) < slowStatusEvery || c.busy) {
		c.mu.Unlock()
		return v
	}
	if v == nil || v.ver != s.rcache.ver.Load() { // nothing yet, or something changed: work it out now
		c.mu.Unlock()
		fresh := s.computeSlowStatus()
		c.mu.Lock()
		c.v = fresh
		c.mu.Unlock()
		return fresh
	}
	c.busy = true
	c.mu.Unlock()
	go func() {
		fresh := s.computeSlowStatus()
		c.mu.Lock()
		c.v, c.busy = fresh, false
		c.mu.Unlock()
	}()
	return v
}
