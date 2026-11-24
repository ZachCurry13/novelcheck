package api

import (
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/discover"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// What the Kindle app (kindle-app/) shows from the web app's Discover and Collections pages: only books the family
// already has and that can be downloaded to the Kindle (a listed book nobody owns has nothing to send), with the
// kids' rules as everywhere else. Collections include the AI-made ones.

type discoverRow struct {
	Title string      `json:"title"`
	Icon  string      `json:"icon"`
	Books []pluginHit `json:"books"`
}

// handleKindleDiscover: GET /kosync/plugin/discover?device= gives Discover's rows with the books in the library.
func (s *Server) handleKindleDiscover(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	device := strings.TrimSpace(r.URL.Query().Get("device"))
	items, err := s.Store.DiscoverItems(u)
	if err != nil {
		kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "couldn't read Discover"})
		return
	}
	newIn, _ := s.Store.NewInLibraries(u, 20)
	family, _ := s.Store.FamilyFavorites(u, 20)
	rows, _ := discover.Compose(items, newIn, family, u, true) // true: the books the family has stay in
	rows = append(s.seasonRows(u), rows...)
	out := []discoverRow{}
	for _, row := range rows {
		var books []store.Book
		for _, d := range row.Books {
			if d.Owned || d.ID > 0 && s.Store.Owned(d.ID) {
				books = append(books, d.Book)
			}
		}
		hits := s.hits(u, device, books)
		if len(hits) > 0 {
			out = append(out, discoverRow{Title: row.Title, Icon: row.Icon, Books: hits})
		}
	}
	kosyncJSON(w, http.StatusOK, map[string]any{"rows": out})
}

// handleKindleCollections: GET /kosync/plugin/collections lists the collections (the family's own and the AI-made
// ones); GET /kosync/plugin/collection/{id}?device= lists one's books that can be downloaded.
func (s *Server) handleKindleCollections(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	cs, err := s.Store.ListCollections(u)
	if err != nil {
		kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "couldn't read the collections"})
		return
	}
	type card struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Icon  string `json:"icon"`
		About string `json:"about"`
		AI    bool   `json:"ai"`
		Books int    `json:"books"`
	}
	out := []card{}
	for _, c := range cs {
		if c.Kind == "idea" || c.Books == 0 { // an idea the family hasn't kept yet is not a collection
			continue
		}
		out = append(out, card{c.ID, c.Name, c.Icon, c.Description, c.Kind == "ai", c.Books})
	}
	kosyncJSON(w, http.StatusOK, map[string]any{"collections": out})
}

func (s *Server) handleKindleCollection(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	id, ok := pathID(r, "id")
	c, cerr := s.Store.CollectionByID(id, u) // kids' limits apply
	if !ok || cerr != nil || c == nil {
		kosyncJSON(w, http.StatusNotFound, map[string]string{"message": "No such collection."})
		return
	}
	books, _, berr := s.Store.ListBooks(store.BookFilter{Collection: id, Limit: 200, Sort: "title"}, u)
	if berr != nil {
		kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "couldn't read the collection"})
		return
	}
	kosyncJSON(w, http.StatusOK, map[string]any{"name": c.Name, "icon": c.Icon, "about": c.Description,
		"books": s.hits(u, strings.TrimSpace(r.URL.Query().Get("device")), books)})
}
