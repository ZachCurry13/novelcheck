package api

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// serveStatic serves the embedded web app. Unknown non-API paths fall back to
// index.html so client-side routes work. The service worker and manifest are
// served uncached so PWA updates roll out promptly.
func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	if st, err := fs.Stat(s.Web, name); err != nil || st.IsDir() {
		name = "index.html"
	}
	switch {
	case name == "sw.js" || name == "manifest.json" || strings.HasSuffix(name, ".html"):
		w.Header().Set("Cache-Control", "no-cache")
	default:
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	if name == "manifest.json" {
		w.Header().Set("Content-Type", "application/manifest+json")
	}
	http.ServeFileFS(w, r, s.Web, name)
}
