package api

import (
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Each person's own choices: how NovelCheck looks, and recent searches.

// handleAppearance saves the theme, font and motion ({"theme", "font", "motion"}).
func (s *Server) handleAppearance(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Theme  string `json:"theme"`
		Font   string `json:"font"`
		Motion string `json:"motion"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	if !store.Themes[body.Theme] || !store.Fonts[body.Font] || !store.Motions[body.Motion] {
		writeErr(w, http.StatusBadRequest, "unknown appearance choice")
		return
	}
	if err := s.Store.SetAppearance(auth.UserFrom(r).ID, body.Theme, body.Font, body.Motion); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) handleSearches(w http.ResponseWriter, r *http.Request) {
	qs, err := s.Store.Searches(auth.UserFrom(r).ID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, qs)
}

// handleAddSearch remembers a Library search ({"query"}).
func (s *Server) handleAddSearch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query string `json:"query"`
	}
	if !readJSON(w, r, &body, 1<<10) {
		return
	}
	u := auth.UserFrom(r)
	if err := s.Store.AddSearch(u.ID, body.Query); err != nil {
		writeStoreErr(w, err)
		return
	}
	s.handleSearches(w, r)
}

func (s *Server) handleClearSearches(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.ClearSearches(auth.UserFrom(r).ID); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, []string{})
}
