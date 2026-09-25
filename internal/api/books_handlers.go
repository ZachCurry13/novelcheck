package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/delivery"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// handleListBooks serves the unified dashboard across every catalog.
// Query params: q, catalog, overlap_with, multi, classification, flags,
// exclude, status, age, format, spice, sort, limit, offset.
func (s *Server) handleListBooks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.BookFilter{
		Query:          q.Get("q"),
		CatalogID:      queryID(r, "catalog"),
		OverlapWith:    queryID(r, "overlap_with"),
		MultiCatalog:   q.Get("multi") == "1",
		DeepOnly:       q.Get("deep") == "1",
		Classification: q.Get("classification"),
		Flags:          splitCSV(q.Get("flags")),
		ExcludeFlags:   splitCSV(q.Get("exclude")),
		Status:         q.Get("status"),
		Age:            q.Get("age"),
		Format:         q.Get("format"),
		Spice:          q.Get("spice"),
		Sort:           q.Get("sort"),
		Limit:          queryInt(r, "limit"),
		Offset:         queryInt(r, "offset"),
	}
	books, total, err := s.Store.ListBooks(f, auth.UserFrom(r))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if books == nil {
		books = []store.Book{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"books": books, "total": total})
}

func (s *Server) handleGetBook(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	u := auth.UserFrom(r)
	b, err := s.Store.BookByID(id, u)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	copies, err := s.Store.BookCopies(id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if !u.IsAdmin() {
		for i := range copies {
			copies[i].Path = "" // don't expose server paths to restricted users
		}
	}
	_, downloadable := delivery.BestFile(copies, false)
	notes, err := s.Store.BookNotes(id, u)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	out := map[string]any{"book": b, "copies": copies, "downloadable": downloadable, "notes": notes,
		"my_delete_request": s.Store.MyDeleteRequest(id, u.ID), "my_wish": s.Store.MyWish(id, u.ID), "owned": s.Store.Owned(id)}
	if u.Role == store.RoleAdmin || u.Role == store.RoleEditor {
		out["calibre_web_url"] = s.Store.Setting(store.KeyCalibreWebURL) // "Open in Calibre-Web" links
	}
	writeJSON(w, http.StatusOK, out)
}

// handleDownload streams the best on-disk copy (used by KOReader / manual sync).
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.Store.BookByID(id, auth.UserFrom(r)); err != nil {
		writeStoreErr(w, err)
		return
	}
	copies, err := s.Store.BookCopies(id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	best, ok := delivery.BestFile(copies, false)
	if !ok || !s.insideCalibre(best.Path) {
		writeErr(w, http.StatusNotFound, "no downloadable file on this server")
		return
	}
	f, err := os.Open(best.Path)
	if err != nil {
		writeErr(w, http.StatusNotFound, "file unavailable")
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		writeErr(w, http.StatusNotFound, "file unavailable")
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+sanitizeFilename(filepath.Base(best.Path))+`"`)
	http.ServeContent(w, r, "", st.ModTime(), f)
}

// insideCalibre guards against serving anything outside the library mount.
func (s *Server) insideCalibre(p string) bool {
	rel, err := filepath.Rel(filepath.Clean(s.Cfg.CalibreDir), filepath.Clean(p))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func sanitizeFilename(n string) string {
	return strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r < 32 {
			return '_'
		}
		return r
	}, n)
}

func (s *Server) handleAnalyzeBook(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	b, err := s.Store.BookByID(id, nil)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if b.Status == "processing" {
		writeErr(w, http.StatusConflict, "analysis already in progress")
		return
	}
	s.Worker.Enqueue(true, id)
	writeJSON(w, http.StatusAccepted, map[string]any{"queued": id})
}
