package api

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/covers"
)

// Review covers (Admin → Library cleanup): go through the books with no cover one at a time, see cleaner titles
// and cover candidates, and pick.

// handleCoverSuggest: GET /api/admin/covers/suggest?id=N[&q=…] gives the book's title and author, cleaner titles
// to search under, and up to 8 cover candidates (searched under q instead when given).
func (s *Server) handleCoverSuggest(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	b, err := s.Store.BookByID(id, nil)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	titles := covers.Titles(b.Title)
	clean := ""
	if len(titles) > 0 && titles[0] != strings.TrimSpace(b.Title) {
		clean = titles[0]
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": b.ID, "title": b.Title, "author": b.Author, "clean_title": clean,
		"candidates": s.Covers.Candidates(ctx, b.Title, b.Author, r.URL.Query().Get("q"), 8)})
}

// handleCoverPreview: GET /api/admin/covers/preview?u=… shows a candidate image through NovelCheck (the page may
// only load its own images), for the two catalogues Review covers searches.
func (s *Server) handleCoverPreview(w http.ResponseWriter, r *http.Request) {
	u := r.URL.Query().Get("u")
	if !covers.AllowedImage(u) {
		writeErr(w, http.StatusBadRequest, "not a cover address")
		return
	}
	hc := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(req *http.Request, _ []*http.Request) error {
		if !covers.AllowedImage(req.URL.String()) {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	resp, err := hc.Get(u)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "the cover could not be fetched")
		return
	}
	defer resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(ct, "image/") {
		writeErr(w, http.StatusNotFound, "no image there")
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 2<<20))
}

// handleCoverChoose: POST /api/admin/covers/choose {"id", "url"} makes that image the book's cover in NovelCheck.
func (s *Server) handleCoverChoose(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID  int64  `json:"id"`
		URL string `json:"url"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	if _, err := s.Store.BookByID(body.ID, nil); err != nil {
		writeStoreErr(w, err)
		return
	}
	if !covers.AllowedImage(body.URL) {
		writeErr(w, http.StatusBadRequest, "not a cover address")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if !s.Covers.Choose(ctx, s.coverName(body.ID), body.URL) {
		writeErr(w, http.StatusBadGateway, "that cover could not be saved; try another")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
