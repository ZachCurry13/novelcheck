package api

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// "Try the new rules on 20 books" (Admin → AI & Scans, with the re-rate banner): a sample of the family's books is
// rated again by the main AI, one at a time, and nothing is saved. The answers sit beside the ratings the books
// have, so a new set of rules can be judged on the real AI before the whole library is re-rated.

type trialRow struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Author     string `json:"author"`
	OldLevel   *int   `json:"old_level"`
	OldSure    string `json:"old_confidence"`
	NewLevel   *int   `json:"new_level"`
	NewSure    string `json:"new_confidence"`
	NewReason  string `json:"new_reason"`
	Warnings   string `json:"warnings"`
	Error      string `json:"error,omitempty"`
	Difference string `json:"difference"` // same, higher, lower
}

type trialJob struct {
	mu      sync.Mutex
	Running bool       `json:"running"`
	Total   int        `json:"total"`
	Results []trialRow `json:"results"`
	At      string     `json:"at"`
}

func (j *trialJob) snapshot() trialJob {
	j.mu.Lock()
	defer j.mu.Unlock()
	return trialJob{Running: j.Running, Total: j.Total, Results: append([]trialRow(nil), j.Results...), At: j.At}
}

// handleTrial: GET /api/admin/rerate/trial (the state and answers so far), POST starts one ({"count": 20}).
func (s *Server) handleTrial(w http.ResponseWriter, r *http.Request) {
	j := &s.trial
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, j.snapshot())
		return
	}
	var body struct {
		Count int `json:"count"`
	}
	if r.ContentLength > 0 && !readJSON(w, r, &body, 1<<10) {
		return
	}
	n := body.Count
	if n < 1 || n > 40 {
		n = 20
	}
	j.mu.Lock()
	if j.Running {
		j.mu.Unlock()
		writeErr(w, http.StatusConflict, "a trial is already running")
		return
	}
	books, err := s.Store.TrialSample(n)
	if err != nil || len(books) == 0 {
		j.mu.Unlock()
		writeErr(w, http.StatusBadRequest, "there are no AI-rated books to try it on yet")
		return
	}
	j.Running, j.Total, j.Results, j.At = true, len(books), nil, time.Now().Format("2 Jan 15:04")
	j.mu.Unlock()
	go func() {
		defer func() { j.mu.Lock(); j.Running = false; j.mu.Unlock() }()
		for _, b := range books {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			row := trialRow{ID: b.ID, Title: b.Title, Author: b.Author, OldLevel: b.Level, OldSure: b.Confidence, Warnings: b.Warnings, Difference: "same"}
			a, err := s.Worker.TrialRate(ctx, b.ID)
			cancel()
			switch {
			case err != nil:
				row.Error = err.Error()
			default:
				row.NewLevel, row.NewSure, row.NewReason = a.SpiceLevel, a.Confidence, a.SpiceReason
				if a.SpiceLevel != nil && b.Level != nil {
					switch {
					case *a.SpiceLevel > *b.Level:
						row.Difference = "higher"
					case *a.SpiceLevel < *b.Level:
						row.Difference = "lower"
					}
				}
			}
			j.mu.Lock()
			j.Results = append(j.Results, row)
			j.mu.Unlock()
		}
	}()
	writeJSON(w, http.StatusAccepted, map[string]int{"total": len(books)})
}
