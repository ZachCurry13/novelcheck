package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

// Which libraries the Library page shows by default, per person ("Save as my default" next to the Library chips). Kept as a list of
// catalog numbers; a library that was deleted since is left out when it is read.

// libDefault is what is kept: the libraries ticked and the Digital / Physical switch.
type libDefault struct {
	Catalogs []int64 `json:"catalogs"`
	Medium   string  `json:"medium,omitempty"`
}

func libDefaultKey(userID int64) string { return "library_default_" + strconv.FormatInt(userID, 10) }

// handleLibraryDefault: GET shows the saved libraries, PUT {"catalogs": [ids]} saves them (an empty list clears the default).
func (s *Server) handleLibraryDefault(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	if r.Method == http.MethodPut {
		var body struct {
			Catalogs []int64 `json:"catalogs"`
			Medium   string  `json:"medium"`
		}
		if !readJSON(w, r, &body, 2<<10) {
			return
		}
		raw := ""
		if body.Medium != "digital" && body.Medium != "physical" {
			body.Medium = ""
		}
		if len(body.Catalogs) > 0 || body.Medium != "" {
			b, _ := json.Marshal(libDefault{Catalogs: body.Catalogs, Medium: body.Medium})
			raw = string(b)
		}
		if err := s.Store.SetSetting(libDefaultKey(u.ID), raw); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	var saved libDefault
	raw := s.Store.Setting(libDefaultKey(u.ID))
	if json.Unmarshal([]byte(raw), &saved) != nil { // before the Digital / Physical switch it was a plain list of numbers
		_ = json.Unmarshal([]byte(raw), &saved.Catalogs)
	}
	ids := saved.Catalogs
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
	writeJSON(w, http.StatusOK, map[string]any{"catalogs": out, "medium": saved.Medium})
}
