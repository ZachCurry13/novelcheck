package api

import (
	"net/http"
)

// Logs from the KOReader plugin's Developer tools (Send log to NovelCheck),
// read in Admin → System & Toggles → Device logs.

// handlePluginDebug: POST /kosync/plugin/debug {"device", "version", "text"}.
func (s *Server) handlePluginDebug(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		Device  string `json:"device"`
		Version string `json:"version"`
		Text    string `json:"text"`
	}
	if !readJSON(w, r, &body, 256<<10) {
		return
	}
	if err := s.Store.SaveDeviceLog(u, body.Device, body.Version, body.Text); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeviceLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := s.Store.DeviceLogs()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs})
}

func (s *Server) handleDeviceLog(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if r.Method == http.MethodDelete {
		if err := s.Store.DeleteDeviceLog(id); err != nil {
			writeStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	text, err := s.Store.DeviceLogText(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "no such log")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(text))
}
