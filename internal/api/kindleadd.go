package api

import (
	"encoding/json"
	"net/http"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// autoAddFromDevice puts the library books on a device into someone's Up Next
// (when they have that switched on): started ones as Reading, the rest queued
// at the end, finished ones skipped. A book they already have there, hidden
// or not, is left alone, and kids' rules still decide what a kid may see.
// It says how many were added.
func (s *Server) autoAddFromDevice(u *store.User, device string) int {
	if !s.Store.KindleAutoAdd(u.ID) || !s.Store.SettingBool(store.KeyModuleQueue) {
		return 0
	}
	progress := s.Store.ReadingProgress(u.ID)
	added := 0
	for _, id := range s.Store.DeviceBookIDs(u.ID, device) {
		if _, err := s.Store.BookByID(id, u); err != nil {
			continue
		}
		status := "queued"
		if p, ok := progress[id]; ok {
			if p.Percent >= store.FinishedAt {
				continue
			}
			if p.Percent > 0 {
				status = "reading"
			}
		}
		if s.Store.EnqueueAuto(u.ID, id, status) {
			added++
		}
	}
	return added
}

// handleKindleAutoAdd turns "add the books on my e-reader to Up Next" on or off.
func (s *Server) handleKindleAutoAdd(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request")
		return
	}
	u := auth.UserFrom(r)
	if err := s.Store.SetKindleAutoAdd(u.ID, body.Enabled); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": body.Enabled})
}
