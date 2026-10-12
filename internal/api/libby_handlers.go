package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/libby"
)

// Libby (Settings → Libby library, and a line on each book): is the book at
// the person's library, how many holds, and a link to it there. Holds
// themselves are placed in Libby: NovelCheck can't sign in for anyone.

func (s *Server) handleLibbyLibrary(w http.ResponseWriter, r *http.Request) {
	key, name := s.Store.LibbyLibrary(auth.UserFrom(r).ID)
	writeJSON(w, http.StatusOK, map[string]string{"library": key, "name": name})
}

// handleLibbySetLibrary: PUT /api/me/libby {"library": "lapl" or "lapl.overdrive.com"}; "" clears it.
func (s *Server) handleLibbySetLibrary(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Library string `json:"library"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	u := auth.UserFrom(r)
	if body.Library == "" {
		_ = s.Store.SetLibbyLibrary(u.ID, "", "")
		writeJSON(w, http.StatusOK, map[string]string{"library": "", "name": ""})
		return
	}
	key := libby.CleanKey(body.Library)
	if key == "" {
		writeErr(w, http.StatusBadRequest, "type your library's key: the first part of its OverDrive address, like lapl for lapl.overdrive.com")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	name, err := s.Libby.LibraryName(ctx, key)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	_ = s.Store.SetLibbyLibrary(u.ID, key, name)
	writeJSON(w, http.StatusOK, map[string]string{"library": key, "name": name})
}

// handleBookLibby: GET /api/books/{id}/libby.
func (s *Server) handleBookLibby(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	key, name := s.Store.LibbyLibrary(u.ID)
	if key == "" {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	b, err := s.Store.BookByID(id, u) // kids' rules apply
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	a, err := s.Libby.Check(ctx, key, b.Title, b.Author)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"configured": true, "library": name, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"configured": true, "library": name, "availability": a})
}

// handleLibbyCheck: POST /api/libby/check {"ids": [...]} (up to 60): the
// library's answer for each book, for the chips on Discover. A few books are
// looked up at a time; answers are kept for a few hours.
func (s *Server) handleLibbyCheck(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if !readJSON(w, r, &body, 8<<10) {
		return
	}
	u := auth.UserFrom(r)
	key, _ := s.Store.LibbyLibrary(u.ID)
	if key == "" {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	if len(body.IDs) > 60 {
		body.IDs = body.IDs[:60]
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	out := map[int64]libby.Availability{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	gate := make(chan struct{}, 5)
	for _, id := range body.IDs {
		b, err := s.Store.BookByID(id, u) // kids' rules apply
		if err != nil {
			continue
		}
		wg.Add(1)
		gate <- struct{}{}
		go func() {
			defer func() { <-gate; wg.Done() }()
			if a, err := s.Libby.Check(ctx, key, b.Title, b.Author); err == nil {
				mu.Lock()
				out[b.ID] = a
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	writeJSON(w, http.StatusOK, map[string]any{"configured": true, "books": out})
}
