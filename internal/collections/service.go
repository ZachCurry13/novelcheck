package collections

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/safe"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Service runs AI fills in the background (a local AI can take a minute or
// two, longer than a request through Cloudflare may last) and makes the
// weekly collection ideas.
type Service struct {
	Store *store.Store
	// Allowed reports whether background AI work may run now (automatic
	// rating's hours; the worker idle). Nil means always.
	Allowed func() bool
	// Local reports whether the main AI is local, for the ideas' default.
	Local func() bool

	mu   sync.Mutex
	jobs map[string]*Job
}

// Job is one AI fill in progress or done.
type Job struct {
	Status  string `json:"status"` // running | done | error
	Picks   []Pick `json:"picks"`
	Error   string `json:"error,omitempty"`
	started time.Time
}

// New makes the service.
func New(st *store.Store) *Service {
	return &Service{Store: st, jobs: map[string]*Job{}}
}

// Start begins filling theme (skipping exclude) and returns the job id.
func (s *Service) Start(theme string, exclude []int64) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	job := &Job{Status: "running", started: time.Now()}
	s.mu.Lock()
	for k, j := range s.jobs { // forget jobs nobody collected within an hour
		if time.Since(j.started) > time.Hour {
			delete(s.jobs, k)
		}
	}
	s.jobs[id] = job
	s.mu.Unlock()
	safe.Go("AI collection", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		picks, err := Fill(ctx, s.Store, theme, exclude)
		s.mu.Lock()
		defer s.mu.Unlock()
		if err != nil {
			job.Status, job.Error = "error", err.Error()
			return
		}
		job.Status, job.Picks = "done", picks
	})
	return id
}

// Job returns a fill's progress (nil if unknown).
func (s *Service) Job(id string) *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return nil
	}
	cp := *j
	return &cp
}
