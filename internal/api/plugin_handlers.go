package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/version"
	kindleplugin "github.com/zachcurry13/novelcheck/kindle-plugin"
)

// The KOReader plugin (kindle-plugin/novelcheck.koplugin) talks to NovelCheck
// in JSON with KOReader's progress-sync sign-in (x-auth-user and x-auth-key,
// the MD5 of the sync code). It lists the reader's Nightstand and downloads
// books from it, with kids' rules applied as everywhere else.

type pluginBook struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
	Format string `json:"format"`
	Size   int64  `json:"size"`
	Level  *int   `json:"level"`
	Status string `json:"status"` // queued | reading
	URL    string `json:"url"`
	// What the Kindle app shows beside a book (the plugin ignores them).
	Series string   `json:"series,omitempty"` // "The Lost Files #1"
	Stars  float64  `json:"stars,omitempty"`  // Hardcover readers' average
	Genres []string `json:"genres,omitempty"`
}

// handlePluginUpNext: GET /kosync/plugin/up-next.
func (s *Server) handlePluginUpNext(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	items, _ := s.Store.ListQueue(u.ID, false)
	books := []pluginBook{}
	for _, it := range items {
		b, err := s.Store.BookByID(it.BookID, u)
		if err != nil {
			continue
		}
		pb, ok := s.pluginBookFor(b)
		if !ok {
			continue // nothing to download
		}
		pb.Status = it.Status
		books = append(books, pb)
	}
	kosyncJSON(w, http.StatusOK, map[string]any{"user": u.Username, "books": books})
}

// handlePluginBook: GET /kosync/plugin/book/{id}/{name}.
func (s *Server) handlePluginBook(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	_ = chi.URLParam(r, "name") // only for a readable file name in the address
	s.serveBook(w, r, u, id)
}

// handlePluginDevice: POST /kosync/plugin/device {"device", "books": [...]}.
// The plugin reports what's on the e-reader: file names (never folders),
// formats, sizes and KOReader's file fingerprints.
func (s *Server) handlePluginDevice(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		Device string             `json:"device"`
		Plugin string             `json:"plugin_version"`
		Books  []store.DeviceBook `json:"books"`
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	if err == nil {
		err = json.Unmarshal(cleanJSONStrings(raw), &body)
	}
	if err != nil {
		setDetail(w, err) // the bug log then says why (a file name with a raw control character, a body cut short, too large)
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "bad request"})
		return
	}
	device := strings.TrimSpace(body.Device)
	if device == "" || len(device) > 60 || len(body.Books) > 3000 {
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "a device name (up to 60 characters) and up to 3000 books"})
		return
	}
	for i := range body.Books {
		b := &body.Books[i]
		b.MD5, b.Format = strings.ToLower(strings.TrimSpace(b.MD5)), strings.ToLower(b.Format)
		b.Author = store.StripJunkToken(b.Author)                             // an identifier Amazon puts after an author
		b.Filename = filepath.Base(strings.ReplaceAll(b.Filename, "\\", "/")) // never a folder
		if len(b.Title) > 300 {
			b.Title = b.Title[:300]
		}
	}
	s.Store.SaveDeviceInfo(u.ID, device, strings.TrimSpace(body.Plugin))
	s.Store.AutoLink(u.ID, device)              // the plugin and the app on one Kindle, when their names say so
	if strings.HasPrefix(body.Plugin, "app ") { // the Kindle app: Amazon store books join the person's store library
		s.addStoreBooks(u, body.Books)
	}
	before := s.Store.DownloadedIDs(u.ID, device)
	matched, err := s.Store.SaveDeviceBooks(u.ID, device, body.Books)
	if err != nil {
		log.Printf("plugin device report for user %d: %v", u.ID, err)
		setDetail(w, err)
		kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "could not save"})
		return
	}
	added := s.autoAddFromDevice(u, device)
	left := []store.LeftBook{}
	if len(body.Books) < 500 || strings.HasPrefix(body.Plugin, "app ") { // the plugin lists 500 books at most: a cut list isn't books leaving
		if l := s.Store.NoteLeft(u.ID, device, before, s.Store.DownloadedIDs(u.ID, device)); l != nil {
			left = l
		}
	}
	kosyncJSON(w, http.StatusOK, map[string]any{"books": len(body.Books), "matched": matched, "added": added, "left": left})
}

// handlePluginVersion tells the plugin which version this NovelCheck has and
// which files make it up, so the plugin can offer to update itself.
func (s *Server) handlePluginVersion(w http.ResponseWriter, r *http.Request) {
	if s.kosyncUser(w, r) == nil {
		return
	}
	kosyncJSON(w, http.StatusOK, map[string]any{"version": kindleplugin.Version(), "files": kindleplugin.UpdatableFiles(), "server": version.Version, "home": s.homeAddress()})
}

// handlePluginFile sends one of the plugin's Lua files.
func (s *Server) handlePluginFile(w http.ResponseWriter, r *http.Request) {
	if s.kosyncUser(w, r) == nil {
		return
	}
	data, ok := kindleplugin.File(chi.URLParam(r, "name"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

// cleanJSONStrings turns a raw control character inside a JSON string (a tab or line break in a file name or a title, which an
// e-reader may leave unescaped) into a space, because Go's decoder refuses the whole report for it. Outside strings nothing changes.
func cleanJSONStrings(b []byte) []byte {
	in, esc := false, false
	for i, c := range b {
		switch {
		case esc:
			esc = false
		case in && c == '\\':
			esc = true
		case c == '"':
			in = !in
		case in && c < 0x20:
			b[i] = ' '
		}
	}
	return b
}
