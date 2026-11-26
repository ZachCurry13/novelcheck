package api

import (
	"fmt"
	"net/http"
)

// What is running on the server right now, for the admin's Home ("Running now"). Every one of these keeps going on the server
// whatever the phone does: closing the app or leaving the page never stops them.

type runningTask struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
	Done   int    `json:"done,omitempty"`
	Total  int    `json:"total,omitempty"`
}

func (s *Server) runningTasks() []runningTask {
	out := []runningTask{}
	if cv := s.covers.state(); cv.Running {
		out = append(out, runningTask{"covers", "Finding covers", cv.Current, cv.Checked, cv.Total})
	}
	if h := s.hcjob.state(); h.Running {
		title := "Comparing the library with Hardcover"
		if h.Kind == "apply" {
			title = "Writing Hardcover fixes into Calibre"
		}
		out = append(out, runningTask{"hardcover", title, h.Note, h.Done, h.Total})
	}
	if !absCleaning.TryLock() {
		out = append(out, runningTask{ID: "audiobooks", Title: "Checking audiobooks against Hardcover"})
	} else {
		absCleaning.Unlock()
	}
	if !s.refreshMu.TryLock() {
		out = append(out, runningTask{ID: "calibre", Title: "Reading Calibre"})
	} else {
		s.refreshMu.Unlock()
	}
	if f := s.formats.state(); f.Running {
		out = append(out, runningTask{"formats", "Calibre format job (" + f.Kind + ")", f.Note, f.Done, f.Total})
	}
	if reads, err := s.Store.DeepReads(50); err == nil {
		for _, d := range reads {
			if d.Status == "reading" {
				out = append(out, runningTask{fmt.Sprint("deep", d.ID), "Deep Scan: " + d.Title, d.Model, d.PartsDone, d.PartsTotal})
			}
		}
	}
	if ws := s.Worker.Status(); ws.State == "analyzing" || ws.State == "enriching" {
		detail := ws.CurrentName
		if ws.QueueLength > 0 {
			detail += fmt.Sprintf(" · %d waiting", ws.QueueLength)
		}
		out = append(out, runningTask{ID: "rating", Title: "Rating books", Detail: detail})
	}
	return out
}

// handleRunning: GET /api/admin/running.
func (s *Server) handleRunning(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"running": s.runningTasks()})
}
