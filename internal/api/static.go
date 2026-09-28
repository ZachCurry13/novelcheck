package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// serveStatic serves the embedded web app. Unknown non-API paths fall back to
// index.html so client-side routes work.
//
// index.html points at versioned addresses (/v/<build>/js/app.js): the build
// is a hash of every app file, so a phone can never run a mix of old and new
// files after an update (a new module importing something an old one lacks
// stops the whole app). Relative imports inherit the prefix, so every module
// of one version shares it. Versioned files never change and cache for a
// year; everything else carries an ETag and is revalidated on every load
// (a 304 when unchanged). Icons and vendored libraries cache for an hour.
func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p, build := stripBuild(r.URL.Path)
	name := strings.TrimPrefix(path.Clean(p), "/")
	if name == "" {
		name = "index.html"
	}
	if st, err := fs.Stat(s.Web, name); err != nil || st.IsDir() {
		name = "index.html"
	}
	switch {
	case name == "index.html":
		s.serveIndex(w, r)
		return
	case build != "" && build == s.buildID():
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	case strings.HasPrefix(name, "icons/") || strings.HasPrefix(name, "vendor/"):
		w.Header().Set("Cache-Control", "public, max-age=3600")
	default:
		w.Header().Set("Cache-Control", "no-cache")
	}
	if tag := s.etag(name); tag != "" {
		w.Header().Set("ETag", tag)
	}
	if name == "manifest.json" {
		w.Header().Set("Content-Type", "application/manifest+json")
	}
	http.ServeFileFS(w, r, s.Web, name)
}

// stripBuild turns /v/<build>/js/app.js into /js/app.js and <build>.
func stripBuild(p string) (string, string) {
	rest, ok := strings.CutPrefix(p, "/v/")
	if !ok {
		return p, ""
	}
	build, file, ok := strings.Cut(rest, "/")
	if !ok || build == "" {
		return p, ""
	}
	return "/" + file, build
}

// serveIndex sends index.html with its scripts and stylesheet at this
// build's versioned addresses.
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	s.indexOnce.Do(func() {
		raw, err := fs.ReadFile(s.Web, "index.html")
		if err != nil {
			return
		}
		prefix := "/v/" + s.buildID()
		for _, attr := range []string{`href="`, `src="`} {
			for _, dir := range []string{"/css/", "/js/", "/vendor/"} {
				raw = bytes.ReplaceAll(raw, []byte(attr+dir), []byte(attr+prefix+dir))
			}
		}
		s.index = raw
	})
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("ETag", `"`+s.buildID()+`"`)
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(s.index))
}

// buildID is a short hash of every embedded file.
func (s *Server) buildID() string {
	s.buildOnce.Do(func() {
		h := sha256.New()
		_ = fs.WalkDir(s.Web, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			data, err := fs.ReadFile(s.Web, p)
			if err == nil {
				_, _ = io.WriteString(h, p)
				_, _ = h.Write(data)
			}
			return nil
		})
		s.build = hex.EncodeToString(h.Sum(nil)[:6])
	})
	return s.build
}

// etag hashes a file once; embedded files never change while running.
func (s *Server) etag(name string) string {
	if tag, ok := s.etags.Load(name); ok {
		return tag.(string)
	}
	data, err := fs.ReadFile(s.Web, name)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	tag := `"` + hex.EncodeToString(sum[:8]) + `"`
	s.etags.Store(name, tag)
	return tag
}
