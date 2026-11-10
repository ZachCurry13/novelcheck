package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zachcurry13/novelcheck/internal/auth"
)

// Reading a photo can take longer than the 100 seconds Cloudflare lets a request run (it then answers 524 and the
// page gives up). With ?async=1 the check runs on its own and the page asks for the answer every couple of seconds.
// It runs as if the person's request were still open, but doesn't stop if their connection drops.

type checkJob struct {
	user   int64
	at     time.Time
	done   bool
	status int
	body   json.RawMessage
}

var (
	checkJobsMu sync.Mutex
	checkJobs   = map[string]*checkJob{}
)

func (s *Server) startCheckJob(w http.ResponseWriter, r *http.Request) {
	buf, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPhoto*4/3+4096))
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "that photo is too large")
		return
	}
	var raw [8]byte
	_, _ = rand.Read(raw[:])
	id := hex.EncodeToString(raw[:])
	job := &checkJob{user: auth.UserFrom(r).ID, at: time.Now()}
	checkJobsMu.Lock()
	for k, j := range checkJobs { // forget old answers
		if time.Since(j.at) > 20*time.Minute {
			delete(checkJobs, k)
		}
	}
	if len(checkJobs) >= 200 {
		checkJobsMu.Unlock()
		writeErr(w, http.StatusTooManyRequests, "too many checks at once: try again in a minute")
		return
	}
	checkJobs[id] = job
	checkJobsMu.Unlock()

	r2 := r.Clone(context.WithoutCancel(r.Context()))
	r2.URL.RawQuery = ""
	r2.Body = io.NopCloser(bytes.NewReader(buf))
	go func() {
		rec := httptest.NewRecorder()
		s.handleCheck(rec, r2)
		checkJobsMu.Lock()
		job.status, job.body, job.done = rec.Code, append(json.RawMessage{}, rec.Body.Bytes()...), true
		checkJobsMu.Unlock()
	}()
	writeJSON(w, http.StatusAccepted, map[string]string{"job": id})
}

// handleCheckJob: GET /api/check/jobs/{id}: {"done": false}, or the finished answer as {"done": true, "status", "body"}.
func (s *Server) handleCheckJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	checkJobsMu.Lock()
	job := checkJobs[chi.URLParam(r, "id")]
	var out map[string]any
	if job != nil && job.user == auth.UserFrom(r).ID {
		out = map[string]any{"done": job.done}
		if job.done {
			out["status"], out["body"] = job.status, job.body
		}
	}
	checkJobsMu.Unlock()
	if out == nil {
		writeErr(w, http.StatusNotFound, "that check is no longer kept: start it again")
		return
	}
	writeJSON(w, http.StatusOK, out)
}
