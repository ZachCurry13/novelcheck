package api

import (
	"net/url"
	"strings"
)

// homeAddress checks the home address a person gives for their Kindle (the server's address on the home network,
// which the plugin tries first: plugin homeaddr.lua): http or https, a host and maybe a port, nothing more. ""
// clears it.
func homeAddress(raw string) (string, bool) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return "", true
	}
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 100 || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" ||
		u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	return u.Scheme + "://" + u.Host, true
}
