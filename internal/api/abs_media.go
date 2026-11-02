package api

// Audiobooks: playing them in the browser and downloading them. The audio comes
// from Audiobookshelf through NovelCheck, so the server's address and tokens never
// reach a browser and the kids' content rules apply to every file.

import (
	"context"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zachcurry13/novelcheck/internal/audiobookshelf"
	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

var inoRE = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// audioBook finds the book and its Audiobookshelf item for a request, writing the error itself.
func (s *Server) audioBook(w http.ResponseWriter, r *http.Request) (*store.Book, string, *audiobookshelf.Client, bool) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return nil, "", nil, false
	}
	u := auth.UserFrom(r)
	b, err := s.Store.BookByID(id, u) // kids' rules apply
	if err != nil {
		writeStoreErr(w, err)
		return nil, "", nil, false
	}
	item := s.Store.AudioItemID(id)
	c, configured := s.absFor(u)
	if item == "" || !configured {
		writeErr(w, http.StatusNotFound, "there's no audiobook of this in the library")
		return nil, "", nil, false
	}
	return b, item, c, true
}

func absErr(w http.ResponseWriter, err error) {
	code := http.StatusBadGateway
	if errors.Is(err, audiobookshelf.ErrSignedOut) {
		code = http.StatusForbidden
	}
	writeErr(w, code, err.Error())
}

// proxy streams an Audiobookshelf file to the browser, passing Range requests through (seeking).
func proxy(w http.ResponseWriter, r *http.Request, c *audiobookshelf.Client, path, filename string) {
	resp, err := c.Do(r.Context(), http.MethodGet, path, nil, r.Header, true)
	if err != nil {
		absErr(w, err)
		return
	}
	defer resp.Body.Close()
	for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Last-Modified", "ETag"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	if filename != "" {
		w.Header().Set("Content-Disposition", `attachment; filename="`+sanitizeFilename(filename)+`"`)
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

var audioExt = map[string]string{"audio/mpeg": ".mp3", "audio/flac": ".flac", "audio/ogg": ".ogg", "audio/x-m4a": ".m4a"}

// handleAudioStream: GET /api/books/{id}/audio/{ino}, one audio file of the book.
func (s *Server) handleAudioStream(w http.ResponseWriter, r *http.Request) {
	_, item, c, ok := s.audioBook(w, r)
	ino := chi.URLParam(r, "ino")
	if !ok {
		return
	}
	if !inoRE.MatchString(ino) {
		writeErr(w, http.StatusBadRequest, "invalid file")
		return
	}
	proxy(w, r, c, "/api/items/"+item+"/file/"+ino, "")
}

// serveAudioDownload: GET /api/books/{id}/download?format=audio. One audio file as it is, or a zip of the folder.
func (s *Server) serveAudioDownload(w http.ResponseWriter, r *http.Request) {
	b, item, c, ok := s.audioBook(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	d, err := c.Detail(ctx, item)
	cancel()
	if err != nil {
		absErr(w, err)
		return
	}
	name := strings.TrimSpace(b.Title)
	if len(d.Tracks) == 1 && inoRE.MatchString(d.Tracks[0].Ino) {
		ext, found := audioExt[d.Tracks[0].Mime]
		if !found {
			ext = ".m4b"
		}
		proxy(w, r, c, "/api/items/"+item+"/file/"+d.Tracks[0].Ino+"/download", name+ext)
		return
	}
	proxy(w, r, c, "/api/items/"+item+"/download", name+".zip")
}

// handleListen: GET /api/books/{id}/listen, what the player needs: files, chapters, the person's place and bookmarks.
func (s *Server) handleListen(w http.ResponseWriter, r *http.Request) {
	b, item, c, ok := s.audioBook(w, r)
	if !ok {
		return
	}
	d, err := c.Detail(r.Context(), item)
	if err != nil {
		absErr(w, err)
		return
	}
	if d.Duration == 0 {
		d.Duration = float64(b.DurationSec)
	}
	writeJSON(w, http.StatusOK, map[string]any{"title": b.Title, "author": b.Author, "narrator": b.Narrator, "detail": d})
}

// handleListenProgress: PUT /api/books/{id}/listen/progress {"at", "duration", "finished"}.
func (s *Server) handleListenProgress(w http.ResponseWriter, r *http.Request) {
	_, item, c, ok := s.audioBook(w, r)
	if !ok {
		return
	}
	var body struct {
		At       float64 `json:"at"`
		Duration float64 `json:"duration"`
		Finished bool    `json:"finished"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	if err := c.SaveProgress(r.Context(), item, max(body.At, 0), body.Duration, body.Finished); err != nil {
		absErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleListenBookmark: POST adds {"time", "title"}, DELETE ?time= removes.
func (s *Server) handleListenBookmark(w http.ResponseWriter, r *http.Request) {
	_, item, c, ok := s.audioBook(w, r)
	if !ok {
		return
	}
	var err error
	if r.Method == http.MethodDelete {
		err = c.DeleteBookmark(r.Context(), item, queryFloat(r, "time"))
	} else {
		var body struct {
			Time  float64 `json:"time"`
			Title string  `json:"title"`
		}
		if !readJSON(w, r, &body, 2<<10) {
			return
		}
		if strings.TrimSpace(body.Title) == "" {
			body.Title = "Bookmark"
		}
		err = c.AddBookmark(r.Context(), item, body.Time, body.Title)
	}
	if err != nil {
		absErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
