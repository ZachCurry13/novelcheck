package api

import (
	"net/http"
	"net/url"
	"strings"
)

// The home address: NovelCheck's address on the home network (like http://192.168.1.20:8080), set once by an admin. The Kindle app
// and the KOReader plugin hear of it when they check for updates and try it first (a short knock), then fall back to the main
// address, so at home they use the Wi-Fi alone and anywhere else the internet address.

const keyHomeAddress = "home_address"

// homeAddress is the address on the home network, or "".
func (s *Server) homeAddress() string { return s.Store.Setting(keyHomeAddress) }

// cleanHomeAddress makes "192.168.1.20:8080" into "http://192.168.1.20:8080"; "" for something that is not an address.
func cleanHomeAddress(raw string) string {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return ""
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Path != "" && u.Path != "/") || len(raw) > 100 {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// handleAdminHomeAddress: GET shows it, PUT {"address"} sets it ("" turns it off).
func (s *Server) handleAdminHomeAddress(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		var body struct {
			Address string `json:"address"`
		}
		if !readJSON(w, r, &body, 1<<10) {
			return
		}
		addr := cleanHomeAddress(body.Address)
		if strings.TrimSpace(body.Address) != "" && addr == "" {
			writeErr(w, http.StatusBadRequest, "that is not an address; use something like http://192.168.1.20:8080")
			return
		}
		if err := s.Store.SetSetting(keyHomeAddress, addr); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"address": s.homeAddress()})
}
