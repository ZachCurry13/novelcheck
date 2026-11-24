package api

import (
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/auth"
)

// handleDeviceLink: PUT /api/me/device-link {"device", "kindle"} says one of your e-reader reports (the KOReader
// plugin's or the Kindle app's) is part of the Kindle called kindle; "" makes it its own again.
func (s *Server) handleDeviceLink(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	var body struct {
		Device string `json:"device"`
		Kindle string `json:"kindle"`
	}
	if !readJSON(w, r, &body, 2<<10) {
		return
	}
	if !s.Store.HasDevice(u.ID, body.Device) || len(strings.TrimSpace(body.Kindle)) > 60 {
		writeErr(w, http.StatusBadRequest, "pick one of your e-readers, and a name up to 60 characters")
		return
	}
	if err := s.Store.LinkDevice(u.ID, body.Device, body.Kindle); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
