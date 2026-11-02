package api

// Audiobookshelf, the admin side and each person's own link. The address and
// token are saved with the other settings (Admin → Delivery & Services); this
// is the status line, the test and the sync button, plus "link my account".

import (
	"context"
	"net/http"
	"time"

	"github.com/zachcurry13/novelcheck/internal/audiobookshelf"
	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// absFor is the connection to use for this person: their own account when
// they linked one, otherwise the shared token. ok is false until an admin set it up.
func (s *Server) absFor(u *store.User) (*audiobookshelf.Client, bool) {
	c, ok := audiobookshelf.FromSettings(s.Store)
	if ok && u != nil {
		if tok, _ := s.Store.ABSLink(u.ID); tok != "" {
			c.Token = tok
		}
	}
	return c, ok
}

func (s *Server) handleABSStatus(w http.ResponseWriter, r *http.Request) {
	_, ok := audiobookshelf.FromSettings(s.Store)
	writeJSON(w, http.StatusOK, map[string]any{"configured": ok, "last_sync": s.Store.Setting(store.KeyABSLastSync),
		"result": s.Store.Setting(store.KeyABSLastResult), "busy": audiobookshelf.Busy(), "audiobooks": s.Store.AudioCount()})
}

// handleABSTest: POST /api/admin/audiobookshelf/test {"url", "token"}, the form as typed (a masked token means the saved one).
func (s *Server) handleABSTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	if body.Token == "" || body.Token == secretMask {
		body.Token = s.Store.Setting(store.KeyABSToken)
	}
	base := audiobookshelf.CleanBase(body.URL)
	if base == "" || body.Token == "" {
		writeErr(w, http.StatusBadRequest, "fill in the server address and the API token first")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	libs, err := (&audiobookshelf.Client{Base: base, Token: body.Token}).Libraries(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	names := []string{}
	for _, l := range libs {
		if l.MediaType == "book" {
			names = append(names, l.Name)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"libraries": names})
}

// handleABSSync: POST /api/admin/audiobookshelf/sync reads the server in the background.
func (s *Server) handleABSSync(w http.ResponseWriter, r *http.Request) {
	c, ok := audiobookshelf.FromSettings(s.Store)
	if !ok {
		writeErr(w, http.StatusBadRequest, "save the server address and API token first")
		return
	}
	if audiobookshelf.Busy() {
		writeErr(w, http.StatusConflict, "a sync is already running")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		res, err := audiobookshelf.Sync(ctx, s.Store, c)
		audiobookshelf.Record(s.Store, res, err)
	}()
	writeJSON(w, http.StatusAccepted, map[string]bool{"started": true})
}

// handleMyABS: GET /api/me/audiobookshelf.
func (s *Server) handleMyABS(w http.ResponseWriter, r *http.Request) {
	_, ok := audiobookshelf.FromSettings(s.Store)
	_, name := s.Store.ABSLink(auth.UserFrom(r).ID)
	writeJSON(w, http.StatusOK, map[string]any{"available": ok, "linked": name != "", "name": name})
}

// handleLinkABS: PUT /api/me/audiobookshelf {"username", "password"} signs in once; only the token is kept.
func (s *Server) handleLinkABS(w http.ResponseWriter, r *http.Request) {
	c, ok := audiobookshelf.FromSettings(s.Store)
	if !ok {
		writeErr(w, http.StatusBadRequest, "an admin hasn't set up Audiobookshelf yet")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	tok, name, err := audiobookshelf.Login(ctx, c.Base, body.Username, body.Password)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if tok == "" {
		writeErr(w, http.StatusBadGateway, "Audiobookshelf didn't hand back a token")
		return
	}
	if err := s.Store.SetABSLink(auth.UserFrom(r).ID, tok, name); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"available": true, "linked": true, "name": name})
}

// handleUnlinkABS: DELETE /api/me/audiobookshelf goes back to the shared account.
func (s *Server) handleUnlinkABS(w http.ResponseWriter, r *http.Request) {
	_ = s.Store.SetABSLink(auth.UserFrom(r).ID, "", "")
	writeJSON(w, http.StatusOK, map[string]any{"available": true, "linked": false, "name": ""})
}
