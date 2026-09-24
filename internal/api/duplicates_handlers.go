package api

import (
	"net/http"
	"os"
	"strconv"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleDuplicates lists books that are in Calibre more than once, with each
// entry's formats and file sizes and a suggested copy to keep.
func (s *Server) handleDuplicates(w http.ResponseWriter, r *http.Request) {
	groups, err := s.Store.CalibreDuplicates()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	isAdmin := auth.UserFrom(r).IsAdmin()
	type group struct {
		store.DupGroup
		Keep string `json:"keep"`
	}
	out := make([]group, 0, len(groups))
	extra := 0
	for _, g := range groups {
		for i := range g.Entries {
			for j := range g.Entries[i].Files {
				f := &g.Entries[i].Files[j]
				if s.insideCalibre(f.Path) {
					if st, err := os.Stat(f.Path); err == nil {
						f.Size = st.Size()
					}
				}
				if !isAdmin {
					f.Path = "" // server paths are admin-only
				}
			}
		}
		extra += len(g.Entries) - 1
		out = append(out, group{g, store.SuggestKeep(g.Entries)})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"groups":     out,
		"extra":      extra,
		"can_remove": isAdmin && s.Store.Setting(store.KeyCalibreSrvURL) != "",
		"synced_at":  s.Store.Setting(store.KeyCalibreLastSync), // when NovelCheck last read Calibre
	})
}

// handleRemoveDuplicates removes the chosen Calibre entries. Each must be a
// current duplicate, and at least one entry of every book is always kept.
func (s *Server) handleRemoveDuplicates(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Remove []string `json:"remove"`
	}
	if !readJSON(w, r, &body, 64<<10) {
		return
	}
	if s.Store.Setting(store.KeyCalibreSrvURL) == "" {
		writeErr(w, http.StatusBadRequest, "set up the calibre Content server connection first")
		return
	}
	if len(body.Remove) == 0 {
		writeErr(w, http.StatusBadRequest, "pick at least one copy to remove")
		return
	}
	groups, err := s.Store.CalibreDuplicates()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	owner := map[string]int{} // calibre id -> group index
	for gi, g := range groups {
		for _, e := range g.Entries {
			owner[e.CalibreID] = gi
		}
	}
	removing := map[int]int{} // group index -> entries being removed
	want := map[int]string{}
	var ids []int
	for _, raw := range body.Remove {
		gi, ok := owner[raw]
		id, err := strconv.Atoi(raw)
		if !ok || err != nil {
			writeErr(w, http.StatusConflict, "calibre book #"+raw+" is no longer a duplicate; refresh the list. Nothing was removed.")
			return
		}
		if _, dup := want[id]; dup {
			continue
		}
		removing[gi]++
		if removing[gi] >= len(groups[gi].Entries) {
			writeErr(w, http.StatusBadRequest, "keep at least one copy of \""+groups[gi].Title+"\". Nothing was removed.")
			return
		}
		want[id] = groups[gi].Title
		ids = append(ids, id)
	}
	removed, skipped, ok := s.removeFromCalibre(w, r, ids, want)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"removed": removed, "skipped": skipped})
}
