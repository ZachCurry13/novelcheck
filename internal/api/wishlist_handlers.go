package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleAddWish puts a book the family doesn't have on the person's wishlist.
func (s *Server) handleAddWish(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	u := auth.UserFrom(r)
	if _, err := s.Store.BookByID(id, u); err != nil {
		writeStoreErr(w, err)
		return
	}
	var body struct {
		Note   string `json:"note"`
		Format string `json:"format"` // "", "audio" or "ebook": the same title in another format
	}
	if r.ContentLength > 0 && !readJSON(w, r, &body, 4<<10) {
		return
	}
	if body.Format == "" && s.Store.Owned(id) {
		writeErr(w, http.StatusBadRequest, "this book is already in your library")
		return
	}
	if body.Format != "" && s.Store.HasFormat(id, body.Format) {
		writeErr(w, http.StatusBadRequest, "the family already has it in that format")
		return
	}
	if err := s.Store.AddWishFormat(id, u, body.Note, body.Format); err != nil {
		if errors.Is(err, store.ErrAlreadyWished) {
			writeErr(w, http.StatusConflict, err.Error())
			return
		}
		writeStoreErr(w, err)
		return
	}
	s.Store.Notify("info", "wishlist", "New books are on the family wishlist.", "#/wishlist")
	writeJSON(w, http.StatusOK, map[string]any{"wish": s.Store.MyWishFormat(id, u.ID, body.Format)})
}

func (s *Server) handleRemoveWish(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.Store.RemoveWishFormat(id, auth.UserFrom(r).ID, r.URL.Query().Get("format")); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleWishlist lists the wishlist: everyone's for parents, one's own otherwise.
func (s *Server) handleWishlist(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	manager := u.Role == store.RoleAdmin || u.Role == store.RoleEditor
	items, err := s.Store.Wishes(u.ID, manager)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "manager": manager})
}

// handleDecideWish lets a parent approve & track, decline, or mark a wish acquired.
func (s *Server) handleDecideWish(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	done, err := s.Store.DecideWish(id, chi.URLParam(r, "action"), auth.UserFrom(r).Username)
	switch {
	case err != nil:
		writeErr(w, http.StatusBadRequest, err.Error())
	case !done:
		writeErr(w, http.StatusConflict, "that wish has already been handled")
	default:
		if s.Store.PendingWishes() == 0 {
			s.Store.Resolve("wishlist")
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
