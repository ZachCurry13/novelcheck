package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Every failed request (status 400 and up, except "sign in" and "not found")
// is kept with its real cause and a short code. The code is sent back in the
// X-Error-Id header (and in the plugin's JSON errors), so what a person saw
// ("E-3f2a") points at one line in the bug report.

type errRecorder struct {
	http.ResponseWriter
	status int
	id     string
	detail string
	body   []byte
}

func (e *errRecorder) assign() string {
	if e.id == "" {
		b := make([]byte, 2)
		_, _ = rand.Read(b)
		e.id = "E-" + hex.EncodeToString(b)
	}
	return e.id
}

func (e *errRecorder) WriteHeader(status int) {
	if e.status == 0 {
		e.status = status
		if recordable(status) {
			e.ResponseWriter.Header().Set("X-Error-Id", e.assign())
		}
	}
	e.ResponseWriter.WriteHeader(status)
}

func (e *errRecorder) Write(p []byte) (int, error) {
	if e.status == 0 {
		e.WriteHeader(http.StatusOK)
	}
	if recordable(e.status) && len(e.body) < 300 {
		e.body = append(e.body, p[:min(len(p), 300-len(e.body))]...)
	}
	return e.ResponseWriter.Write(p)
}

func (e *errRecorder) Flush() {
	if f, ok := e.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (e *errRecorder) Unwrap() http.ResponseWriter { return e.ResponseWriter }

func (e *errRecorder) SetDetail(d string) { e.detail = d }

func recordable(status int) bool {
	return status >= 400 && status != http.StatusUnauthorized && status != http.StatusNotFound
}

// setDetail tells the recorder the real cause of an error response.
func setDetail(w http.ResponseWriter, err error) {
	if d, ok := w.(interface{ SetDetail(string) }); ok && err != nil {
		d.SetDetail(err.Error())
	}
}

// errorID is this response's code (made now, so it can go into a JSON body).
func errorID(w http.ResponseWriter) string {
	if e, ok := w.(*errRecorder); ok {
		return e.assign()
	}
	return ""
}

func (s *Server) errorRecorder(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &errRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if recordable(rec.status) {
			s.noteError(rec, r, rec.status)
		}
	})
}

func (s *Server) noteError(rec *errRecorder, r *http.Request, status int) {
	detail := rec.detail
	if detail == "" {
		detail = strings.TrimSpace(string(rec.body))
	} else if len(rec.body) > 0 {
		detail += " · response: " + strings.TrimSpace(string(rec.body))
	}
	user := r.Header.Get("x-auth-user")
	s.Store.LogError(store.ServerError{Code: rec.assign(), Method: r.Method, Path: r.URL.Path, Status: status, Username: user, Detail: detail})
}
