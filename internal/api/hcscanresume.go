package api

import (
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// A scan that ends because Calibre or Hardcover stopped answering (an app restarting, Hardcover's limit) is tried again by
// itself, up to four times, half an hour apart, from where it stopped; the suggestions found so far are kept.
const (
	scanRetries   = 4
	scanRetryWait = 30 * time.Minute
	scanBatch     = 25
)

// scanRetryable says whether the scan's closing note is about a service that may be back soon.
func scanRetryable(note string) bool {
	for _, w := range []string{"didn't answer", "can't reach", "too many requests"} {
		if strings.Contains(note, w) {
			return true
		}
	}
	return false
}

// wait keeps the scan "running" while it rests (so Stop shows and nobody starts a second scan) and says why.
func (j *hcJob) wait(note string) {
	j.update(func(x *hcJobState) { x.Running, x.Note = true, note })
}

// resume goes on from done books (counts and suggestions kept).
func (j *hcJob) resume(done int) {
	j.update(func(x *hcJobState) { x.Running, x.Done, x.Note = true, done, "" })
}

func (s *Server) runHCScanResuming(token string, refs []store.CalibreBookRef) {
	s.scanPart(token, refs, false)
	for try := 0; try < scanRetries; try++ {
		st := s.hcjob.state()
		if st.Done >= st.Total || s.hcjob.stopped() || !scanRetryable(st.Note) {
			return
		}
		s.hcjob.wait(st.Note + " · it tries again in 30 minutes by itself")
		for end := time.Now().Add(scanRetryWait); time.Now().Before(end); time.Sleep(10 * time.Second) {
			if s.hcjob.stopped() {
				s.hcEnd("Stopped; what was found so far is below.")
				return
			}
		}
		from := st.Done / scanBatch * scanBatch // back to the start of the group that was cut short
		if from >= len(refs) {
			s.hcEnd("Done.")
			return
		}
		s.hcjob.resume(from)
		s.scanPart(token, refs[from:], true)
	}
}
