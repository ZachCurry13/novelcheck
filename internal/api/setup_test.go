package api_test

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/calibre"
	"github.com/zachcurry13/novelcheck/internal/config"
	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/sysinfo"
	"github.com/zachcurry13/novelcheck/internal/tunnel"
)

// A fresh install with no NOVELCHECK_ADMIN_PASSWORD: the first visitor creates
// the admin in the browser, and setup is locked afterwards.
func TestFirstRunSetup(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	if err := auth.Bootstrap(st, "admin", ""); err != nil {
		t.Fatal(err)
	}
	if n, _ := st.CountUsers(); n != 0 {
		t.Fatal("bootstrap without a password must not create an account")
	}
	s := &api.Server{
		Cfg: config.Config{CalibreDir: t.TempDir(), SessionDays: 1}, Store: st,
		Auth: &auth.Manager{Store: st, SessionDays: 1}, Worker: analyzer.New(st),
		Syncer:  &calibre.Syncer{Store: st, Dir: t.TempDir()},
		Web:     fstest.MapFS{"index.html": {Data: []byte("app")}},
		Tunnel:  &tunnel.Manager{Binary: "no-such-cloudflared"},
		SysInfo: sysinfo.New(t.TempDir()),
	}
	srv := httptest.NewServer(s.Router())
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	c := &client{t: t, base: srv.URL, http: &http.Client{Jar: jar}}

	if _, out := c.do("GET", "/api/setup", nil, false); out["needed"] != true {
		t.Fatalf("setup should be needed: %v", out)
	}
	if res, _ := c.do("POST", "/api/setup", map[string]string{"username": "zach", "password": "short"}, true); res.StatusCode != 400 {
		t.Fatalf("weak password accepted: %d", res.StatusCode)
	}
	res, u := c.do("POST", "/api/setup", map[string]string{"username": "zach", "password": "goodpass123"}, true)
	if res.StatusCode != 201 || u["role"] != "admin" {
		t.Fatalf("setup: %d %v", res.StatusCode, u)
	}
	if res, _ := c.do("GET", "/api/admin/settings", nil, false); res.StatusCode != 200 {
		t.Fatalf("new admin should be signed in: %d", res.StatusCode)
	}
	if _, out := c.do("GET", "/api/setup", nil, false); out["needed"] != false {
		t.Fatal("setup should be done")
	}
	other := &client{t: t, base: srv.URL, http: &http.Client{}}
	if res, _ := other.do("POST", "/api/setup", map[string]string{"username": "evil", "password": "evilpass123"}, true); res.StatusCode != 409 {
		t.Fatalf("second setup must be refused: %d", res.StatusCode)
	}
}
