package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
	kindleapp "github.com/zachcurry13/novelcheck/kindle-app"
)

// handleKindleLibrary: GET /api/me/kindle-library?kindle=<name>: what one Kindle holds, with each book's rating (the
// Kindle page). A Kindle is the KOReader plugin's report and the Kindle app's report together (device links); a
// book both report shows once.
func (s *Server) handleKindleLibrary(w http.ResponseWriter, r *http.Request) {
	u, ok := s.taskOwner(w, r) // yours, or (?user=) one of the people you manage
	if !ok {
		return
	}
	kindle := strings.TrimSpace(r.URL.Query().Get("kindle"))
	links := s.Store.DeviceLinks(u.ID)
	out, seen := []store.KindleBook{}, map[string]bool{}
	for _, d := range s.Store.Devices(u.ID) {
		g := d.Name
		if l := links[d.Name]; l != "" {
			g = l
		}
		if g != kindle {
			continue
		}
		for _, b := range s.Store.KindleLibrary(u.ID, d.Name) {
			key := strings.ToLower(b.Title + "|" + b.Author)
			if b.BookID > 0 {
				key = fmt.Sprint("id", b.BookID)
			}
			if !seen[key] {
				seen[key] = true
				if b.BookID > 0 { // a book merged away or removed since the report: no link to a page that says "not found"
					if _, err := s.Store.BookByID(b.BookID, auth.UserFrom(r)); err != nil {
						b.BookID = 0
					}
				}
				out = append(out, b)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"books": out})
}

// handleKindles: GET /api/me/kindles[?user=]: the Kindles that reported (plugin and app reports, their links) for the
// Kindle page.
func (s *Server) handleKindles(w http.ResponseWriter, r *http.Request) {
	u, ok := s.taskOwner(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": s.Store.Devices(u.ID), "links": s.Store.DeviceLinks(u.ID), "app_latest": kindleapp.Version})
}
