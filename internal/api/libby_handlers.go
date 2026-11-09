package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/libby"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Libby (Settings → Libby libraries, and a line on each book): is the book at one of the person's libraries
// (more than one is common), how many holds, and a link to it there. Holds themselves are placed in Libby:
// NovelCheck can't sign in for anyone.

func libbyJSON(libs []store.LibbyLib) map[string]any {
	out := map[string]any{"libraries": libs, "library": "", "name": ""}
	if len(libs) > 0 { // the first, for older code that knew only one
		out["library"], out["name"] = libs[0].Key, libs[0].Name
	}
	return out
}

func (s *Server) handleLibbyLibrary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, libbyJSON(s.Store.LibbyLibraries(auth.UserFrom(r).ID)))
}

// handleLibbySetLibrary: PUT /api/me/libby {"library": "lapl" or "lapl.overdrive.com"} adds a library;
// "" removes them all.
func (s *Server) handleLibbySetLibrary(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Library string `json:"library"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	u := auth.UserFrom(r)
	if body.Library == "" {
		_ = s.Store.ClearLibbyLibraries(u.ID)
		writeJSON(w, http.StatusOK, libbyJSON(nil))
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
	if err := s.Store.AddLibbyLibrary(u.ID, key, name); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, libbyJSON(s.Store.LibbyLibraries(u.ID)))
}

// handleLibbyRemove: DELETE /api/me/libby/{key} removes one library.
func (s *Server) handleLibbyRemove(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	if err := s.Store.RemoveLibbyLibrary(u.ID, libby.CleanKey(chi.URLParam(r, "key"))); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, libbyJSON(s.Store.LibbyLibraries(u.ID)))
}

// better says whether a is a better answer than b: available now, then the shorter wait.
func better(a, b libby.Availability) bool {
	if !b.Found {
		return a.Found
	}
	if !a.Found {
		return false
	}
	if a.Available != b.Available {
		return a.Available
	}
	if a.WaitDays != b.WaitDays {
		return a.WaitDays < b.WaitDays
	}
	return a.Holds < b.Holds
}

// checkLibraries asks each of the person's libraries about one book (e-book and audiobook).
func (s *Server) checkLibraries(ctx context.Context, libs []store.LibbyLib, title, author string) (perLib []map[string]any, bestEbook, bestAudio libby.Availability) {
	for _, l := range libs {
		entry := map[string]any{"key": l.Key, "name": l.Name}
		a, err := s.Libby.Check(ctx, l.Key, title, author)
		if err != nil {
			entry["error"] = err.Error()
		} else {
			a.Library = l.Name
			entry["availability"] = a
			if better(a, bestEbook) {
				bestEbook = a
			}
		}
		if au, err := s.Libby.CheckMedia(ctx, l.Key, title, author, "audiobook"); err == nil && au.Found {
			au.Library = l.Name
			entry["audio"] = au
			if better(au, bestAudio) {
				bestAudio = au
			}
		}
		perLib = append(perLib, entry)
	}
	return
}

// handleBookLibby: GET /api/books/{id}/libby.
func (s *Server) handleBookLibby(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	libs := s.Store.LibbyLibraries(u.ID)
	if len(libs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	b, err := s.Store.BookByID(id, u) // kids' rules apply
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	per, ebook, audio := s.checkLibraries(ctx, libs, b.Title, b.Author)
	out := map[string]any{"configured": true, "libraries": per, "library": libs[0].Name, "availability": ebook}
	if ebook.Found {
		out["library"] = ebook.Library
	}
	failed := 0
	var firstErr any
	for _, e := range per {
		if e["error"] != nil {
			failed++
			if firstErr == nil {
				firstErr = e["error"]
			}
		}
	}
	if failed == len(per) { // not one library could be asked
		delete(out, "availability")
		out["error"] = firstErr
	}
	if audio.Found {
		out["audio"] = audio
	}
	writeJSON(w, http.StatusOK, out)
}

// handleLibbyCheck: POST /api/libby/check {"ids": [...]} (up to 60): the libraries' best answer for each book,
// for the chips on Discover. A few books are looked up at a time; answers are kept for a few hours.
func (s *Server) handleLibbyCheck(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if !readJSON(w, r, &body, 8<<10) {
		return
	}
	u := auth.UserFrom(r)
	libs := s.Store.LibbyLibraries(u.ID)
	if len(libs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	if len(body.IDs) > 60 {
		body.IDs = body.IDs[:60]
	}
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()
	out, audio := map[int64]libby.Availability{}, map[int64]libby.Availability{}
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
			_, ebook, au := s.checkLibraries(ctx, libs, b.Title, b.Author)
			mu.Lock()
			defer mu.Unlock()
			if ebook.Found {
				out[b.ID] = ebook
			}
			if au.Found {
				audio[b.ID] = au
			}
		}()
	}
	wg.Wait()
	writeJSON(w, http.StatusOK, map[string]any{"configured": true, "books": out, "audio": audio})
}

// handleLibbyAudioKept: POST /api/libby/audio {"ids": [...]}: what the background check already found about
// the audiobooks of these books at the person's libraries (no waiting on the library service).
func (s *Server) handleLibbyAudioKept(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if !readJSON(w, r, &body, 16<<10) {
		return
	}
	if len(body.IDs) > 200 {
		body.IDs = body.IDs[:200]
	}
	writeJSON(w, http.StatusOK, map[string]any{"audio": s.Store.LibbyAudioFor(auth.UserFrom(r).ID, body.IDs)})
}
