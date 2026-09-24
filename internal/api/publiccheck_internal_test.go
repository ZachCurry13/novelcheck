package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckPublicAddress(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
		status  string
		msg     string
	}{
		{"novelcheck answers", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/setup" {
				_, _ = w.Write([]byte(`{"needed":false}`))
			}
		}, "ok", "Opens from the internet"},
		{"cloudflare has no route", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Server", "cloudflare")
			w.WriteHeader(404)
		}, "error", "doesn't know where to send"},
		{"tunnel can't reach app", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(502) }, "error", "can't reach NovelCheck"},
		{"some other site", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<html>hello</html>")) }, "error", "isn't NovelCheck"},
	}
	for _, c := range cases {
		srv := httptest.NewServer(c.handler)
		status, msg, _ := checkPublicAddress(context.Background(), srv.URL)
		srv.Close()
		if status != c.status || !strings.Contains(msg, c.msg) {
			t.Errorf("%s: got %s %q", c.name, status, msg)
		}
	}
	if status, msg, _ := checkPublicAddress(context.Background(), "https://no-such-host.invalid"); status != "error" || !strings.Contains(msg, "doesn't exist in DNS") {
		t.Errorf("missing DNS record: %s %q", status, msg)
	}
}
