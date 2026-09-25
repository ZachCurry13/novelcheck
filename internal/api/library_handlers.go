package api

// Managing libraries: rename, private or shared, owner (admins), removing
// a book from your own library, deleting a library.

import (
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// editableCatalog loads a library the signed-in person may change.
func (s *Server) editableCatalog(w http.ResponseWriter, r *http.Request) (*store.Catalog, bool) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return nil, false
	}
	c, err := s.Store.CatalogByID(id)
	if err != nil {
		writeStoreErr(w, err)
		return nil, false
	}
	if !c.CanEdit(auth.UserFrom(r)) {
		writeErr(w, http.StatusForbidden, "only the library's owner or an admin can change it")
		return nil, false
	}
	return c, true
}

// handleUpdateCatalog renames a library, makes it private or shared, or
// (admins) gives it to someone.
func (s *Server) handleUpdateCatalog(w http.ResponseWriter, r *http.Request) {
	c, ok := s.editableCatalog(w, r)
	if !ok {
		return
	}
	var body struct {
		Name    *string `json:"name"`
		Private *bool   `json:"private"`
		OwnerID *int64  `json:"owner_id"` // 0 = the whole family
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	if body.OwnerID != nil {
		if auth.UserFrom(r).Role != store.RoleAdmin {
			writeErr(w, http.StatusForbidden, "only admins can change who owns a library")
			return
		}
		if *body.OwnerID > 0 {
			o, err := s.Store.UserByID(*body.OwnerID)
			if err != nil || o.Role == store.RoleRestricted {
				writeErr(w, http.StatusBadRequest, "choose an adult account (kids' accounts can't own a library)")
				return
			}
		}
		if err := s.Store.SetCatalogOwner(c.ID, *body.OwnerID); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	if body.Private != nil {
		if *body.Private && c.OwnerID == nil && (body.OwnerID == nil || *body.OwnerID == 0) {
			writeErr(w, http.StatusBadRequest, "give the library an owner first; a family library is always shared")
			return
		}
		if err := s.Store.SetCatalogPrivate(c.ID, *body.Private); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	if body.Name != nil {
		name, valid := validCatalogName(*body.Name)
		if !valid || strings.EqualFold(name, store.CalibreCatalogName) || strings.EqualFold(name, store.LookedUpCatalog) {
			writeErr(w, http.StatusBadRequest, "invalid library name")
			return
		}
		if err := s.Store.RenameCatalog(c.ID, name); err != nil {
			writeErr(w, http.StatusConflict, "a library with that name already exists")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleDeleteCatalog deletes a library (its owner or an admin). Books in
// no other library go with it.
func (s *Server) handleDeleteCatalog(w http.ResponseWriter, r *http.Request) {
	c, ok := s.editableCatalog(w, r)
	if !ok {
		return
	}
	if u := auth.UserFrom(r); u.Role != store.RoleAdmin && (c.OwnerID == nil || *c.OwnerID != u.ID) {
		writeErr(w, http.StatusForbidden, "only the library's owner or an admin can delete it")
		return
	}
	if err := s.Store.DeleteCatalog(c.ID); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleRemoveFromCatalog takes a book out of a library its owner (or an
// admin) manages, straight away: no delete request needed.
func (s *Server) handleRemoveFromCatalog(w http.ResponseWriter, r *http.Request) {
	c, ok := s.editableCatalog(w, r)
	if !ok {
		return
	}
	bookID, ok := pathID(r, "book")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid book id")
		return
	}
	removed, err := s.Store.RemoveFromCatalog(c.ID, bookID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if !removed {
		writeErr(w, http.StatusNotFound, "that book isn't in this library")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// editableCatalogs lists the libraries among a book's copies that u may
// take it out of (their own, or any for admins).
func (s *Server) editableCatalogs(copies []store.BookCopy, u *store.User) []int64 {
	out := []int64{}
	seen := map[int64]bool{}
	for _, c := range copies {
		if seen[c.CatalogID] {
			continue
		}
		seen[c.CatalogID] = true
		if cat, err := s.Store.CatalogByID(c.CatalogID); err == nil && cat.CanEdit(u) && cat.Name != store.LookedUpCatalog {
			out = append(out, c.CatalogID)
		}
	}
	return out
}
