package api

import (
	"net/url"
	"testing"
)

// Secrets in an address never reach the debug recording.
func TestSafeAddress(t *testing.T) {
	for in, want := range map[string]string{
		"/api/books?q=dune":                     "/api/books?q=dune",
		"/opds/abc123secret/book/5/x.epub":      "/opds/(token)/book/5/x.epub",
		"/opds/book/5/x.epub":                   "/opds/book/5/x.epub",
		"/api/x?token=abc&q=1":                  "/api/x?q=1&token=%28hidden%29",
		"/api/y?sync_code=blue-duck&password=p": "/api/y?password=%28hidden%29&sync_code=%28hidden%29",
	} {
		u, _ := url.Parse(in)
		if got := safeAddress(u); got != want {
			t.Errorf("%s -> %s, want %s", in, got, want)
		}
	}
}
