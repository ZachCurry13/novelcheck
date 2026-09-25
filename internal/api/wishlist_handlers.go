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
	if s.Store.Owned(id) {
		writeErr(w, http.StatusBadRequest, "this book is already in your library")
		return
	}
	var body struct {
		Note string `json:"note"`
	}
	if r.ContentLength > 0 && !readJSON(w, r, &body, 4<<10) {
		return
	}
	if err := s.Store.AddWish(id, u, body.Note); err != nil {
		if errors.Is(err, store.ErrAlreadyWished) {
			writeErr(w, http.StatusConflict, err.Error())
			return
		}
		writeStoreErr(w, err)
		return
	}
	s.Store.Notify("info", "wishlist", "New books are on the family wishlist.", "#/wishlist")
	writeJSON(w, http.StatusOK, map[string]any{"wish": s.Store.MyWish(id, u.ID)})
}

func (s *Server) handleRemoveWish(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.Store.RemoveWish(id, auth.UserFrom(r).ID); err != nil {
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
