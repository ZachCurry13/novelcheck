package api

import (
	"strconv"
	"strings"
	"sync"
)

// formatJob is format cleanup's one background job at a time: removing
// formats, putting them back, or converting books to EPUB, all through
// calibre's Content server. The Admin page polls its state.
type formatJob struct {
	mu   sync.Mutex
	st   formatJobState
	stop bool
}

type formatJobState struct {
	Kind    string   `json:"kind"` // remove | restore | convert
	Running bool     `json:"running"`
	Total   int      `json:"total"`
	Done    int      `json:"done"`
	Skipped int      `json:"skipped"`
	Failed  int      `json:"failed"`
	Current string   `json:"current"`
	Percent int      `json:"percent"` // the current book's conversion
	Errors  []string `json:"errors"`
	Note    string   `json:"note"` // how it ended
}

// begin starts a job unless one is running.
func (j *formatJob) begin(kind string, total int) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.st.Running {
		return false
	}
	j.st = formatJobState{Kind: kind, Running: true, Total: total, Errors: []string{}}
	j.stop = false
	return true
}

func (j *formatJob) update(fn func(*formatJobState)) {
	j.mu.Lock()
	defer j.mu.Unlock()
	fn(&j.st)
}

// fail counts a file or book that didn't work, keeping the last 20 reasons.
func (j *formatJob) fail(title, why string) {
	j.update(func(st *formatJobState) {
		st.Failed++
		st.Errors = append(st.Errors, title+": "+why)
		if len(st.Errors) > 20 {
			st.Errors = st.Errors[len(st.Errors)-20:]
		}
	})
}

// end finishes the job with a note on how it went.
func (j *formatJob) end(note string) {
	j.update(func(st *formatJobState) {
		st.Running, st.Current, st.Percent, st.Note = false, "", 0, note
	})
}

func (j *formatJob) state() formatJobState {
	j.mu.Lock()
	defer j.mu.Unlock()
	st := j.st
	st.Errors = append([]string{}, j.st.Errors...)
	return st
}

func (j *formatJob) askStop() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.stop = true
}

func (j *formatJob) stopped() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.stop
}

// count is "1 file" or "3 files".
func count(n int, one string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + one + "s"
}

// lastLine is the last non-empty line of calibre's log or error, for a
// short reason a conversion failed.
func lastLine(texts ...string) string {
	for _, t := range texts {
		lines := strings.Split(strings.TrimSpace(t), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			if l := strings.TrimSpace(lines[i]); l != "" {
				if len(l) > 200 {
					l = l[:200] + "…"
				}
				return l
			}
		}
	}
	return "calibre couldn't convert it"
}
