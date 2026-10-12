package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/zachcurry13/novelcheck/internal/delivery"
	"github.com/zachcurry13/novelcheck/internal/store"
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
		copies, _ := s.Store.BookCopies(b.ID)
		best, ok := delivery.BestFile(copies, false)
		if !ok || !s.insideCalibre(best.Path) {
			continue // nothing to download
		}
		format := strings.ToLower(best.Format)
		var size int64
		if st, err := os.Stat(best.Path); err == nil {
			size = st.Size()
		}
		books = append(books, pluginBook{ID: b.ID, Title: b.Title, Author: b.Author, Format: format, Size: size, Level: b.SpiceLevel,
			Status: it.Status, URL: "/kosync/plugin/book/" + strconv.FormatInt(b.ID, 10) + "/" + fileName(b.Title, format)})
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
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&body); err != nil {
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
		b.Filename = filepath.Base(strings.ReplaceAll(b.Filename, "\\", "/")) // never a folder
		if len(b.Title) > 300 {
			b.Title = b.Title[:300]
		}
	}
	s.Store.SaveDeviceInfo(u.ID, device, strings.TrimSpace(body.Plugin))
	matched, err := s.Store.SaveDeviceBooks(u.ID, device, body.Books)
	if err != nil {
		kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "could not save"})
		return
	}
	added := s.autoAddFromDevice(u, device)
	kosyncJSON(w, http.StatusOK, map[string]int{"books": len(body.Books), "matched": matched, "added": added})
}

// handlePluginVersion tells the plugin which version this NovelCheck has and
// which files make it up, so the plugin can offer to update itself.
func (s *Server) handlePluginVersion(w http.ResponseWriter, r *http.Request) {
	if s.kosyncUser(w, r) == nil {
		return
	}
	kosyncJSON(w, http.StatusOK, map[string]any{"version": kindleplugin.Version(), "files": kindleplugin.UpdatableFiles()})
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
