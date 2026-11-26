package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/calibresrv"
	"github.com/zachcurry13/novelcheck/internal/freebooks"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Free books: a book in the public domain (Project Gutenberg) can be put in the family library with one tap, from Check a book, the
// KOReader plugin or the Kindle app. It goes into Calibre the way an upload does (checked with Hardcover), then shows up in the
// library like any book. Only a book whose title and author agree with the catalog's is offered, and only a person who may add
// books can add one (admins, parents and people an admin allowed).

var freeClientOnce sync.Once
var freeClientDefault *freebooks.Client

func (s *Server) freeClient() *freebooks.Client {
	if s.Free != nil {
		return s.Free
	}
	freeClientOnce.Do(func() { freeClientDefault = freebooks.New() })
	return freeClientDefault
}

// freeMatch is the free copy of a book, if any (the answer is kept for hours, so this is cheap to ask again).
func (s *Server) freeMatch(ctx context.Context, title, author string) (freebooks.Match, bool) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second) // Check a book must not wait long for a third party
	defer cancel()
	return s.freeClient().Find(ctx, title, author)
}

// mayAddFree: who may put a free book in the library.
func mayAddFree(u *store.User) bool {
	return u != nil && u.Role != store.RoleFriend && (u.MayUpload() || u.CanManage())
}

// addFree fetches the free copy and adds it to Calibre. It returns the title it was added as.
func (s *Server) addFree(ctx context.Context, u *store.User, title, author string) (string, int, error) {
	if !mayAddFree(u) {
		return "", http.StatusForbidden, errors.New("an admin has to switch on adding books for you")
	}
	if s.Store.Setting(store.KeyCalibreSrvURL) == "" {
		return "", http.StatusBadRequest, errors.New("adding books isn't set up yet: ask an admin")
	}
	if !u.IsAdmin() && s.Store.UploadsToday(u.ID) >= maxUploadsPerDay {
		return "", http.StatusTooManyRequests, errors.New("that's " + strconv.Itoa(maxUploadsPerDay) + " books today: try again tomorrow")
	}
	m, ok := s.freeMatch(ctx, title, author)
	if !ok {
		return "", http.StatusNotFound, errors.New("there is no free copy of that book")
	}
	fctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	data, err := s.freeClient().Fetch(fctx, m)
	if err != nil {
		return "", http.StatusBadGateway, errors.New("the free library couldn't send it: " + err.Error())
	}
	name := sanitizeFilename(strings.TrimSpace(m.Title)) + ".epub"
	cal := s.calibreClient()
	id, err := cal.AddBook(ctx, name, data)
	if errors.Is(err, calibresrv.ErrDuplicate) {
		return m.Title, http.StatusConflict, errors.New("that book is already in the library")
	}
	if err != nil {
		return "", http.StatusBadGateway, errors.New("the library couldn't take it: " + err.Error())
	}
	s.Store.NoteUpload(strconv.Itoa(id), u.ID)
	s.checkUpload(ctx, cal, id) // the title and author as Hardcover has them, where it is sure
	s.syncSoon(nil)             // NovelCheck reads Calibre again so the book shows up
	s.Store.NotifyRoutine("free-book", "Added “"+m.Title+"” (free, public domain) to the library for "+u.Username+".", "#/library")
	return m.Title, http.StatusOK, nil
}

// handleFree: GET /api/free?title=&author= says whether a free copy exists.
func (s *Server) handleFree(w http.ResponseWriter, r *http.Request) {
	m, ok := s.freeMatch(r.Context(), r.URL.Query().Get("title"), r.URL.Query().Get("author"))
	writeJSON(w, http.StatusOK, map[string]any{"free": ok, "match": m})
}

// handleFreeAdd: POST /api/free/add {"title", "author"}.
func (s *Server) handleFreeAdd(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title  string `json:"title"`
		Author string `json:"author"`
	}
	if !readJSON(w, r, &body, 2<<10) {
		return
	}
	u := auth.UserFrom(r)
	title, status, err := s.addFree(r.Context(), u, body.Title, body.Author)
	if err != nil {
		writeErr(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"title": title})
}

// handlePluginFreeAdd: POST /kosync/plugin/free-add {"title", "author"}, the same for the e-reader.
func (s *Server) handlePluginFreeAdd(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		Title  string `json:"title"`
		Author string `json:"author"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<10)).Decode(&body); err != nil {
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "which book?"})
		return
	}
	title, status, err := s.addFree(r.Context(), u, body.Title, body.Author)
	if err != nil {
		kosyncJSON(w, status, map[string]string{"message": err.Error()})
		return
	}
	kosyncJSON(w, http.StatusOK, map[string]string{"title": title})
}

// handlePluginFree: GET /kosync/plugin/free?title=&author= says whether a free copy exists.
func (s *Server) handlePluginFree(w http.ResponseWriter, r *http.Request) {
	if s.kosyncUser(w, r) == nil {
		return
	}
	m, ok := s.freeMatch(r.Context(), r.URL.Query().Get("title"), r.URL.Query().Get("author"))
	kosyncJSON(w, http.StatusOK, map[string]any{"free": ok, "match": m})
}
