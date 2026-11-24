package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

// Library cleanup tells the bug link what it didn't do. cleanupNote is for an action's own words; cleanupRecorder
// watches every change the Library cleanup routes make and notes any that was refused or failed, or that answered
// with skipped or failed items, with the request and the answer, so what is missing can be built in.

func (s *Server) cleanupNote(action, text string) { s.Store.AddCleanupLog(action, text) }

type cleanupCapture struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (c *cleanupCapture) WriteHeader(code int) { c.status = code; c.ResponseWriter.WriteHeader(code) }
func (c *cleanupCapture) Write(b []byte) (int, error) {
	if c.body.Len() < 16<<10 {
		c.body.Write(b)
	}
	return c.ResponseWriter.Write(b)
}

// notDone are the answer's fields that mean something wasn't done.
var notDone = []string{"skipped", "failed", "failures", "errors", "not_in_calibre", "unmatched", "refused", "left"}

func (s *Server) cleanupRecorder(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		var req bytes.Buffer
		if r.Body != nil {
			body := http.MaxBytesReader(w, r.Body, 1<<20)
			_, _ = req.ReadFrom(body)
			r.Body = readCloser{bytes.NewReader(req.Bytes())}
		}
		c := &cleanupCapture{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(c, r)
		why := ""
		if c.status >= 400 {
			why = fmt.Sprintf("answered %d", c.status)
		} else {
			var out map[string]any
			if json.Unmarshal(c.body.Bytes(), &out) == nil {
				var parts []string
				for _, k := range notDone {
					if v, ok := out[k]; ok && !empty(v) {
						parts = append(parts, k)
					}
				}
				if len(parts) > 0 {
					why = "partly done (" + strings.Join(parts, ", ") + ")"
				}
			}
		}
		if why == "" {
			return
		}
		u := auth.UserFrom(r)
		who := ""
		if u != nil {
			who = u.Username + " · "
		}
		s.cleanupNote(r.Method+" "+r.URL.Path, who+why+"\n  asked: "+clipText(req.String(), 1500)+"\n  answer: "+clipText(c.body.String(), 2500))
	})
}

type readCloser struct{ *bytes.Reader }

func (readCloser) Close() error { return nil }

func empty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case float64:
		return x == 0
	case string:
		return x == ""
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	case bool:
		return !x
	}
	return false
}

func clipText(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// writeCleanupLog is the bug link's section.
func (s *Server) writeCleanupLog(b *strings.Builder) {
	list := s.Store.CleanupLog(60)
	fmt.Fprintf(b, "\n== Library cleanup: what wasn't done (newest first, %d) ==\n", len(list))
	for _, e := range list {
		fmt.Fprintf(b, "%s UTC · %s · %s\n", e.At, e.Action, e.Text)
	}
}
