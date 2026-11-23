package api

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/covers"
)

// Change cover (a book's page): anyone picks one of the covers found for the book or uploads their own, and only
// their own view changes; an admin can also use it for everyone, which wins over Calibre's cover. "Back to the
// usual cover" removes the choice. Served by handleCover, the person's choice first, then the family's.

func myCoverName(userID, bookID int64) string {
	return "mine-" + strconv.FormatInt(userID, 10) + "-" + strconv.FormatInt(bookID, 10)
}

func pickedCoverName(bookID int64) string { return "pick-" + strconv.FormatInt(bookID, 10) }

// handleCoverOptions: GET /api/books/{id}/cover-options[?q=…], up to 8 covers found for the book.
func (s *Server) handleCoverOptions(w http.ResponseWriter, r *http.Request) {
	id, _ := pathID(r, "id")
	b, err := s.Store.BookByID(id, auth.UserFrom(r))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, map[string]any{"candidates": s.Covers.Candidates(ctx, b.Title, b.Author, r.URL.Query().Get("q"), 8),
		"can_set_for_everyone": auth.UserFrom(r).IsAdmin()})
}

// handleSetCover: POST /api/books/{id}/cover, JSON {"url", "everyone"} or a form with "image" (and "everyone").
func (s *Server) handleSetCover(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	id, _ := pathID(r, "id")
	if _, err := s.Store.BookByID(id, u); err != nil {
		writeStoreErr(w, err)
		return
	}
	var url string
	var everyone bool
	var upload []byte
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		r.Body = http.MaxBytesReader(w, r.Body, 9<<20)
		f, _, err := r.FormFile("image")
		if err != nil {
			writeErr(w, http.StatusBadRequest, "pick a picture (JPEG or PNG, up to 8 MB)")
			return
		}
		defer f.Close()
		upload, _ = io.ReadAll(f)
		everyone = r.FormValue("everyone") == "true"
	} else {
		var body struct {
			URL      string `json:"url"`
			Everyone bool   `json:"everyone"`
		}
		if !readJSON(w, r, &body, 4<<10) {
			return
		}
		url, everyone = body.URL, body.Everyone
		if !covers.AllowedImage(url) {
			writeErr(w, http.StatusBadRequest, "not a cover address")
			return
		}
	}
	if everyone && !u.IsAdmin() {
		writeErr(w, http.StatusForbidden, "only an admin can change the cover for everyone; your choice changes your own view")
		return
	}
	name := myCoverName(u.ID, id)
	if everyone {
		name = pickedCoverName(id)
	}
	ok := false
	if upload != nil {
		ok = s.Covers.SaveUpload(name, upload)
	} else {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		ok = s.Covers.Choose(ctx, name, url)
	}
	if !ok {
		writeErr(w, http.StatusBadRequest, "that picture couldn't be used (a JPEG or PNG of at least 100 × 100 pixels); try another")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleResetCover: DELETE /api/books/{id}/cover[?everyone=1], back to the usual cover.
func (s *Server) handleResetCover(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	id, _ := pathID(r, "id")
	if r.URL.Query().Get("everyone") != "" {
		if !u.IsAdmin() {
			writeErr(w, http.StatusForbidden, "only an admin can change the cover for everyone")
			return
		}
		s.Covers.Drop(pickedCoverName(id))
	} else {
		s.Covers.Drop(myCoverName(u.ID, id))
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
