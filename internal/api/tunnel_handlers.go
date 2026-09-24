package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/zachcurry13/novelcheck"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// handleTunnelStatus reports the built-in Cloudflare Tunnel's state.
func (s *Server) handleTunnelStatus(w http.ResponseWriter, r *http.Request) {
	st := s.Tunnel.Status()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    st,
		"enabled":   s.Store.SettingBool(store.KeyTunnelEnabled),
		"hostname":  s.Store.Setting(store.KeyTunnelHostname),
		"has_token": s.Store.Setting(store.KeyTunnelToken) != "",
		"guide":     novelcheck.RemoteAccessGuide,
	})
}

// handleTunnelSave stores the tunnel settings and starts or stops the
// connector. An empty token keeps the saved one.
func (s *Server) handleTunnelSave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled  bool   `json:"enabled"`
		Token    string `json:"token"`
		Hostname string `json:"hostname"`
	}
	if !readJSON(w, r, &body, 16<<10) {
		return
	}
	token := strings.TrimSpace(body.Token)
	// People often paste the whole install command; keep just the token.
	if i := strings.LastIndex(token, "--token"); i >= 0 {
		token = strings.TrimSpace(token[i+len("--token"):])
	}
	if f := strings.Fields(token); len(f) > 0 {
		token = f[len(f)-1]
	}
	if len(token) > 4096 || strings.ContainsAny(token, "\r\n") {
		writeErr(w, http.StatusBadRequest, "that doesn't look like a tunnel token")
		return
	}
	host := strings.TrimSpace(body.Hostname)
	host = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://"), "/")
	if len(host) > 253 || strings.ContainsAny(host, " /\\\r\n") {
		writeErr(w, http.StatusBadRequest, "enter just the address, for example books.example.com")
		return
	}
	if token != "" {
		if err := s.Store.SetSetting(store.KeyTunnelToken, token); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	_ = s.Store.SetSetting(store.KeyTunnelHostname, host)
	_ = s.Store.SetSetting(store.KeyTunnelEnabled, strconv.FormatBool(body.Enabled))
	if err := s.Tunnel.Apply(body.Enabled, s.Store.Setting(store.KeyTunnelToken)); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.Tunnel.Status())
}
