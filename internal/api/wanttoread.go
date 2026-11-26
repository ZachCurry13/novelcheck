package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/hardcover"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Want to read: the books on a person's Hardcover "Want to read" shelf, each matched against the family library: ready to
// download, or free (public domain: one tap adds it), or neither (it can be requested). Nothing is added by itself.

type wantItem struct {
	Title        string `json:"title"`
	Author       string `json:"author"`
	ID           int64  `json:"id,omitempty"` // the library book, when it is there
	InLibrary    bool   `json:"in_library"`
	Downloadable bool   `json:"downloadable"` // the library has a file the plugin and the app can send
	Free         bool   `json:"free"`         // a public-domain copy can be added
	Wished       bool   `json:"wished"`
}

const maxWant = 40

// wantList reads the shelf and matches it; connected is false when the person has not connected Hardcover.
func (s *Server) wantList(ctx context.Context, u *store.User) (items []wantItem, connected bool, err error) {
	if s.Store.HardcoverToken(u.ID) == "" {
		return []wantItem{}, false, nil
	}
	shelf, err := s.hardcoverClient(u.ID).Shelves(ctx)
	if err != nil {
		return nil, true, err
	}
	items = []wantItem{}
	for _, b := range shelf {
		if b.Status != hardcover.WantToRead {
			continue
		}
		if len(items) >= maxWant {
			break
		}
		it := wantItem{Title: b.Title, Author: b.Authors}
		if id := s.Store.MatchBook(b.Title, b.Authors); id != 0 && !s.otherAuthor(id, b.Authors) {
			if bk, err := s.Store.BookByID(id, u); err == nil && s.Store.Owned(id) {
				it.ID, it.InLibrary = id, true
				_, it.Downloadable = s.pluginBookFor(bk)
			}
			it.Wished = s.Store.MyWish(id, u.ID) != nil
		}
		items = append(items, it)
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	lctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	for i := range items {
		if items[i].InLibrary || !mayAddFree(u) {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(it *wantItem) {
			defer func() { <-sem; wg.Done() }()
			_, it.Free = s.freeMatch(lctx, it.Title, it.Author)
		}(&items[i])
	}
	wg.Wait()
	return items, true, nil
}

// handleWantToRead: GET /api/me/want-to-read.
func (s *Server) handleWantToRead(w http.ResponseWriter, r *http.Request) {
	items, connected, err := s.wantList(r.Context(), auth.UserFrom(r))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "Hardcover didn't answer: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connected": connected, "books": items})
}

// handlePluginWant: GET /kosync/plugin/want, the same for the e-reader.
func (s *Server) handlePluginWant(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	items, connected, err := s.wantList(r.Context(), u)
	if err != nil {
		kosyncJSON(w, http.StatusBadGateway, map[string]string{"message": "Hardcover didn't answer: " + err.Error()})
		return
	}
	kosyncJSON(w, http.StatusOK, map[string]any{"connected": connected, "books": items})
}
