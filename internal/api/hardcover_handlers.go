package api

import (
	"context"
	"net/http"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/hardcover"
)

// Hardcover (Profile → Hardcover): each person pastes their own token; it's
// tested, kept secret, and their reading and read shelves are pulled in
// hourly or on "Sync now". hardcoverURL points tests at a fake server.
var hardcoverURL = ""

func (s *Server) hardcoverClient(userID int64) *hardcover.Client {
	return &hardcover.Client{Token: s.Store.HardcoverToken(userID), URL: hardcoverURL}
}

func (s *Server) hardcoverState(userID int64) map[string]any {
	synced, books, matched := s.Store.HardcoverStatus(userID)
	return map[string]any{"connected": s.Store.HardcoverToken(userID) != "", "synced_at": synced, "books": books, "matched": matched}
}

// handleHardcoverStatus: GET /api/me/hardcover.
func (s *Server) handleHardcoverStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.hardcoverState(auth.UserFrom(r).ID))
}

// handleHardcoverConnect: PUT /api/me/hardcover {"token"}; an empty token disconnects.
func (s *Server) handleHardcoverConnect(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	u := auth.UserFrom(r)
	token := hardcover.CleanToken(body.Token)
	if token == "" {
		if err := s.Store.SetHardcoverToken(u.ID, ""); err != nil {
			writeStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, s.hardcoverState(u.ID))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	c := &hardcover.Client{Token: token, URL: hardcoverURL}
	name, err := c.Username(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if err := s.Store.SetHardcoverToken(u.ID, token); err != nil {
		writeStoreErr(w, err)
		return
	}
	if _, _, err := hardcover.SyncUser(ctx, s.Store, c, u.ID); err != nil {
		writeErr(w, http.StatusBadGateway, "connected as "+name+", but the first sync failed: "+err.Error())
		return
	}
	out := s.hardcoverState(u.ID)
	out["username"] = name
	writeJSON(w, http.StatusOK, out)
}

// handleHardcoverSync: POST /api/me/hardcover/sync.
func (s *Server) handleHardcoverSync(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	if s.Store.HardcoverToken(u.ID) == "" {
		writeErr(w, http.StatusBadRequest, "connect Hardcover first")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if _, _, err := hardcover.SyncUser(ctx, s.Store, s.hardcoverClient(u.ID), u.ID); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.hardcoverState(u.ID))
}
