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

// Job is one AI fill, or one shelf check, in progress or done. A check's
// Picks are the books that don't fit, with the AI's reason.
type Job struct {
	Status  string `json:"status"` // running | done | error
	Kind    string `json:"kind"`   // fill | check
	Picks   []Pick `json:"picks"`
	Done    int    `json:"done"` // books checked so far
	Total   int    `json:"total"`
	Error   string `json:"error,omitempty"`
	started time.Time
}

// New makes the service.
func New(st *store.Store) *Service {
	return &Service{Store: st, jobs: map[string]*Job{}}
}

// newJob registers a running job and returns its id.
func (s *Service) newJob(job *Job) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	job.Status, job.started = "running", time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, j := range s.jobs { // forget jobs nobody collected within an hour
		if time.Since(j.started) > time.Hour {
			delete(s.jobs, k)
		}
	}
	s.jobs[id] = job
	return id
}

// StartCheck gives a shelf's books the second look in the background.
func (s *Service) StartCheck(theme string, books []store.Book) (string, error) {
	if !withinBudget(s.Store, CheckTokens(len(books))) {
		return "", ErrBudget
	}
	job := &Job{Kind: "check", Total: len(books)}
	id := s.newJob(job)
	safe.Go("shelf check", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		answers, err := Verify(ctx, s.Store, theme, books, func(done int) {
			s.mu.Lock()
			job.Done = done
			s.mu.Unlock()
		})
		misfits := []Pick{}
		for _, b := range books {
			if a, ok := answers[b.ID]; ok && a.Fits == "no" {
				misfits = append(misfits, Pick{Book: b, Reason: a.Reason})
			}
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case err != nil && len(answers) == 0:
			job.Status, job.Error = "error", err.Error()
		case err != nil: // part of the shelf was checked
			job.Status, job.Picks, job.Error = "done", misfits, "Only part of the shelf was checked: "+err.Error()
		default:
			job.Status, job.Picks = "done", misfits
		}
	})
	return id, nil
}

// Start begins filling theme (skipping exclude) and returns the job id.
func (s *Service) Start(theme string, exclude []int64) string {
	job := &Job{Kind: "fill"}
	id := s.newJob(job)
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
