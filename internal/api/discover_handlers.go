package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/discover"
	"github.com/zachcurry13/novelcheck/internal/safe"
	"github.com/zachcurry13/novelcheck/internal/seasons"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleDiscover returns the Discover rows the viewer may see: the outside
// lists (with their content rules; kids never see unrated books) and the
// family's own new and favorite books.
func (s *Server) handleDiscover(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	items, err := s.Store.DiscoverItems(u)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	newIn, _ := s.Store.NewInLibraries(u, 20)
	family, _ := s.Store.FamilyFavorites(u, 20)
	rows, owned := discover.Compose(items, newIn, family, u, r.URL.Query().Get("owned") == "1")
	if rows == nil {
		rows = []discover.Row{}
	}
	rows = append(s.seasonRows(u), rows...)
	writeJSON(w, http.StatusOK, map[string]any{
		"rows":       rows,
		"owned":      owned, // listed books the family has, left out unless ?owned=1
		"nyt":        discover.FromNYT(rows),
		"has_key":    strings.TrimSpace(s.Store.Setting(store.KeyNYTAPIKey)) != "",
		"updated":    s.Store.Setting(store.KeyDiscoverLastRefresh),
		"refreshing": s.Discover != nil && s.Discover.Running(),
	})
}

// seasonRows puts the family's books for the current seasons (at most two
// shelves) at the top of Discover.
func (s *Server) seasonRows(u *store.User) []discover.Row {
	var rows []discover.Row
	for _, se := range seasons.Current(time.Now()) {
		if len(rows) == 2 {
			break
		}
		f := store.BookFilter{Limit: 20, Sort: "recent"}
		s.seasonFilter(&f, se.Key)
		books, _, err := s.Store.ListBooks(f, u)
		if err != nil || len(books) == 0 {
			continue
		}
		row := discover.Row{Key: "season_" + se.Key, Icon: se.Icon, Title: se.Name + " from your libraries"}
		for _, b := range books {
			row.Books = append(row.Books, store.DiscoverBook{Book: b, List: "season", Owned: true, Queued: b.InQueue})
		}
		rows = append(rows, row)
	}
	return rows
}

// handleDiscoverStatus is the admin view: last refresh and how far rating got.
func (s *Server) handleDiscoverStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"updated":    s.Store.Setting(store.KeyDiscoverLastRefresh),
		"result":     s.Store.Setting(store.KeyDiscoverLastResult),
		"has_key":    strings.TrimSpace(s.Store.Setting(store.KeyNYTAPIKey)) != "",
		"daily":      s.Store.SettingInt(store.KeyDiscoverDaily),
		"counts":     s.Store.DiscoverCounts(),
		"refreshing": s.Discover != nil && s.Discover.Running(),
	})
}

// handleDiscoverRefresh fetches the lists now, in the background (the New
// York Times asks for a pause between calls, so it takes about a minute).
func (s *Server) handleDiscoverRefresh(w http.ResponseWriter, r *http.Request) {
	if s.Discover == nil {
		writeErr(w, http.StatusServiceUnavailable, "Discover isn't running")
		return
	}
	if s.Discover.Running() {
		writeErr(w, http.StatusConflict, discover.ErrBusy.Error())
		return
	}
	go func() {
		_ = safe.Run("discover refresh", func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			if _, err := s.Discover.Refresh(ctx); err != nil && !errors.Is(err, discover.ErrBusy) {
				log.Printf("discover refresh: %v", err)
			}
			return nil
		})
	}()
	writeJSON(w, http.StatusAccepted, map[string]bool{"started": true})
}

// handleDiscoverTest checks a New York Times key (the one typed, or the saved one).
func (s *Server) handleDiscoverTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key string `json:"key"`
	}
	if r.ContentLength > 0 && !readJSON(w, r, &body, 1<<10) {
		return
	}
	key := strings.TrimSpace(body.Key)
	if key == "" || key == secretMask {
		key = strings.TrimSpace(s.Store.Setting(store.KeyNYTAPIKey))
	}
	if key == "" {
		writeErr(w, http.StatusBadRequest, "enter a New York Times Books API key first")
		return
	}
	if s.Discover == nil {
		writeErr(w, http.StatusServiceUnavailable, "Discover isn't running")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	n, err := s.Discover.Test(ctx, key)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"books": n})
}
