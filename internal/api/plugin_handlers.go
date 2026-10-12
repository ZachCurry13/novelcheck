package api

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/zachcurry13/novelcheck/internal/delivery"
)

// The KOReader plugin (kindle-plugin/novelcheck.koplugin) talks to NovelCheck
// in JSON with KOReader's progress-sync sign-in (x-auth-user and x-auth-key,
// the MD5 of the sync code). It lists the reader's Up Next and downloads
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
