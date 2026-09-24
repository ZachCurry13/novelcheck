package api

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck"
	"github.com/zachcurry13/novelcheck/internal/calibresrv"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func (s *Server) calibreClient() *calibresrv.Client {
	return &calibresrv.Client{
		URL:      s.Store.Setting(store.KeyCalibreSrvURL),
		Username: s.Store.Setting(store.KeyCalibreSrvUser),
		Password: s.Store.Setting(store.KeyCalibreSrvPassword),
		Library:  s.Store.Setting(store.KeyCalibreSrvLibrary),
	}
}

// handleCalibreServerGet returns the Content server settings (no password).
func (s *Server) handleCalibreServerGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"url":          s.Store.Setting(store.KeyCalibreSrvURL),
		"user":         s.Store.Setting(store.KeyCalibreSrvUser),
		"has_password": s.Store.Setting(store.KeyCalibreSrvPassword) != "",
		"library":      s.Store.Setting(store.KeyCalibreSrvLibrary),
		"configured":   s.Store.Setting(store.KeyCalibreSrvURL) != "",
		"guide":        novelcheck.CalibreServerGuide,
	})
}

// handleCalibreServerSave tests the connection, then saves it. An empty
// password keeps the saved one; an empty address turns the feature off.
func (s *Server) handleCalibreServerSave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL, User, Password, Library string
	}
	if !readJSON(w, r, &body, 8<<10) {
		return
	}
	if strings.TrimSpace(body.URL) == "" {
		for _, k := range []string{store.KeyCalibreSrvURL, store.KeyCalibreSrvUser, store.KeyCalibreSrvPassword, store.KeyCalibreSrvLibrary} {
			_ = s.Store.SetSetting(k, "")
		}
		writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	base, err := calibresrv.Normalize(body.URL)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	pw := body.Password
	if pw == "" {
		pw = s.Store.Setting(store.KeyCalibreSrvPassword)
	}
	c := &calibresrv.Client{URL: base, Username: strings.TrimSpace(body.User), Password: pw}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	libs, def, err := c.Libraries(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	lib := strings.TrimSpace(body.Library)
	if _, ok := libs[lib]; !ok {
		lib = def
	}
	for k, v := range map[string]string{store.KeyCalibreSrvURL: base, store.KeyCalibreSrvUser: c.Username,
		store.KeyCalibreSrvPassword: pw, store.KeyCalibreSrvLibrary: lib} {
		if err := s.Store.SetSetting(k, v); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"configured": true, "libraries": libs, "library": lib})
}

// handleCalibreRemove removes the Calibre books the given hide filters catch,
// through the Content server (to calibre's recycle bin). It recomputes the
// list, refuses if it changed since the admin reviewed it, and checks every
// title against calibre first so the wrong library can never be touched.
func (s *Server) handleCalibreRemove(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Hide           string `json:"hide"`
		Q              string `json:"q"`
		Classification string `json:"classification"`
		ExpectedCount  int    `json:"expected_count"`
	}
	if !readJSON(w, r, &body, 8<<10) {
		return
	}
	if s.Store.Setting(store.KeyCalibreSrvURL) == "" {
		writeErr(w, http.StatusBadRequest, "set up the calibre Content server connection first")
		return
	}
	f := store.BookFilter{Query: body.Q, Classification: body.Classification, AnyFlags: splitCSV(body.Hide)}
	if len(f.AnyFlags) == 0 && f.Classification == "" {
		writeErr(w, http.StatusBadRequest, "tick at least one Hide box first")
		return
	}
	matches, err := s.Store.CalibreMatches(f)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if len(matches) != body.ExpectedCount {
		writeErr(w, http.StatusConflict, "the list changed since you reviewed it; please check it again")
		return
	}
	ids := make([]int, 0, len(matches))
	want := map[int]string{}
	for _, m := range matches {
		if id, err := strconv.Atoi(m.CalibreID); err == nil {
			ids = append(ids, id)
			want[id] = m.Title
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	c := s.calibreClient()
	have, err := c.Titles(ctx, ids)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	var toRemove []int
	skipped := 0
	for _, id := range ids {
		title, ok := have[id]
		if !ok {
			skipped++ // already gone from calibre
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(title), strings.TrimSpace(want[id])) {
			writeErr(w, http.StatusConflict, "stopped: calibre's book #"+strconv.Itoa(id)+" is \""+title+
				"\" but NovelCheck expected \""+want[id]+"\". Is the Content server using the same library? Nothing was removed.")
			return
		}
		toRemove = append(toRemove, id)
	}
	for start := 0; start < len(toRemove); start += 200 {
		end := min(start+200, len(toRemove))
		if err := c.Remove(ctx, toRemove[start:end]); err != nil {
			writeErr(w, http.StatusBadGateway, "removed "+strconv.Itoa(start)+" books, then: "+err.Error())
			return
		}
	}
	log.Printf("calibre: removed %d books via Content server (requested by admin)", len(toRemove))
	go func() {
		if _, err := s.Syncer.Run(); err != nil {
			log.Printf("calibre sync after removal failed: %v", err)
		}
	}()
	writeJSON(w, http.StatusOK, map[string]int{"removed": len(toRemove), "skipped": skipped})
}
