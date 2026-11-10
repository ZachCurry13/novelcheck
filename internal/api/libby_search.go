package api

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/libby"
)

// handleLibbySearch: GET /api/libby/search?q=… looks the words up at each of the person's libraries (e-books and
// audiobooks) whether or not NovelCheck has the book, and merges the answers: one row per book, the best
// answer for each kind. Adding one to the Nightstand is POST /api/libby/add, then POST /api/queue.
func (s *Server) handleLibbySearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	libs := s.Store.LibbyLibraries(auth.UserFrom(r).ID)
	if len(libs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	if len([]rune(q)) < 2 {
		writeJSON(w, http.StatusOK, map[string]any{"configured": true, "books": []libby.Found{}})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	var mu sync.Mutex
	var wg sync.WaitGroup
	merged := map[string]*libby.Found{}
	var order []string
	failed := 0
	for _, l := range libs {
		for _, media := range []string{"ebook", "audiobook"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				found, err := s.Libby.Search(ctx, l.Key, q, media)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					failed++
					return
				}
				for _, f := range found {
					m := merged[f.Key]
					if m == nil {
						c := f
						c.Ebook, c.Audio = nil, nil
						m = &c
						merged[f.Key] = m
						order = append(order, f.Key)
					}
					if m.Cover == "" {
						m.Cover = f.Cover
					}
					if f.Ebook != nil {
						f.Ebook.Library = l.Name
						if m.Ebook == nil || better(*f.Ebook, *m.Ebook) {
							m.Ebook = f.Ebook
						}
					}
					if f.Audio != nil {
						f.Audio.Library = l.Name
						if m.Audio == nil || better(*f.Audio, *m.Audio) {
							m.Audio = f.Audio
						}
					}
				}
			}()
		}
	}
	wg.Wait()
	books := make([]libby.Found, 0, len(order))
	for _, k := range order {
		books = append(books, *merged[k])
	}
	if len(books) > 40 {
		books = books[:40]
	}
	out := map[string]any{"configured": true, "books": books}
	if failed > 0 && len(books) == 0 {
		out["error"] = "Couldn't reach the library service just now."
	}
	writeJSON(w, http.StatusOK, out)
}

// handleLibbyAdd: POST /api/libby/add {"title","author","cover"} makes a book of a search result and returns
// its id (the page then puts it on the Nightstand with POST /api/queue).
func (s *Server) handleLibbyAdd(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title  string `json:"title"`
		Author string `json:"author"`
		Cover  string `json:"cover"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	if strings.TrimSpace(body.Title) == "" {
		writeErr(w, http.StatusBadRequest, "title required")
		return
	}
	id, err := s.Store.AddFromLibby(body.Title, body.Author, body.Cover)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"book_id": id})
}
