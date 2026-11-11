package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

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

// A request that takes this long is kept in the report's slow-request list, so a slow
// tab can be pinned to the page that causes it.
const slowAfter = 800 * time.Millisecond

// changesData says whether a request may change what the list pages show. Reading-progress
// pings and report uploads happen all the time and never do.
func changesData(method, path string) bool {
	if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
		return false
	}
	for _, p := range []string{"/kosync/syncs/", "/bug-report", "/plugin/debug", "/bugs/", "/api/auth/"} {
		if strings.Contains(path, p) {
			return false
		}
	}
	return true
}

// slowExempt is for requests that are slow by nature: file downloads, audio, covers
// being fetched, AI calls, backups and long jobs.
func slowExempt(path string) bool {
	for _, p := range []string{"/download", "/audio/", "/cover", "/file/", "/kosync/plugin/book", "/opds/", "/backup", "/api/check", "/diagnose", "/analyze", "/deep-scan", "/listen", "/formats/", "/calibre-sync", "/libby"} {
		if strings.Contains(path, p) {
			return true
		}
	}
	return false
}

func recordable(status int) bool {
	return status >= 400 && status != http.StatusUnauthorized && status != http.StatusNotFound
}

// setDetail tells the recorder the real cause of an error response.
func setDetail(w http.ResponseWriter, err error) {
	// The recorder may sit under other wrappers (a later middleware wraps the writer again): look through them,
	// or the bug report shows "internal error" with no cause.
	for i := 0; i < 8 && w != nil && err != nil; i++ {
		if d, ok := w.(interface{ SetDetail(string) }); ok {
			d.SetDetail(err.Error())
			return
		}
		u, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return
		}
		w = u.Unwrap()
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
		began := time.Now()
		next.ServeHTTP(rec, r)
		s.noteActivity(r, rec.status, time.Since(began))
		if changesData(r.Method, r.URL.Path) && rec.status > 0 && rec.status < 400 {
			s.rcache.bump() // something changed: the kept list pages are out of date
		}
		if recordable(rec.status) {
			s.noteError(rec, r, rec.status)
		}
		if took := time.Since(began); took >= slowAfter && !slowExempt(r.URL.Path) {
			s.Store.LogError(store.ServerError{Code: "SLOW", Method: r.Method, Path: r.URL.Path, Status: max(rec.status, 200),
				Username: r.Header.Get("x-auth-user"), Detail: strconv.FormatInt(took.Milliseconds(), 10) + " ms"})
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
