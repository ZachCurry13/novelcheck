package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

// Which libraries the Library page shows by default, per person ("Save as my default" next to the Library chips). Kept as a list of
// catalog numbers; a library that was deleted since is left out when it is read.

func libDefaultKey(userID int64) string { return "library_default_" + strconv.FormatInt(userID, 10) }

// handleLibraryDefault: GET shows the saved libraries, PUT {"catalogs": [ids]} saves them (an empty list clears the default).
func (s *Server) handleLibraryDefault(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	if r.Method == http.MethodPut {
		var body struct {
			Catalogs []int64 `json:"catalogs"`
		}
		if !readJSON(w, r, &body, 2<<10) {
			return
		}
		raw := ""
		if len(body.Catalogs) > 0 {
			b, _ := json.Marshal(body.Catalogs)
			raw = string(b)
		}
		if err := s.Store.SetSetting(libDefaultKey(u.ID), raw); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	var ids []int64
	_ = json.Unmarshal([]byte(s.Store.Setting(libDefaultKey(u.ID))), &ids)
	known := map[int64]bool{}
	if cats, err := s.Store.ListCatalogs(u); err == nil {
		for _, c := range cats {
			known[c.ID] = true
		}
	}
	out := []int64{}
	for _, id := range ids {
		if known[id] {
			out = append(out, id)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"catalogs": out})
}
