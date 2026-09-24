package api_test

import (
	"bytes"
	"encoding/json"
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
	"github.com/zachcurry13/novelcheck/internal/push"
	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/sysinfo"
	"github.com/zachcurry13/novelcheck/internal/tunnel"
)

type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func (c *client) do(method, path string, body any, csrf bool) (*http.Response, map[string]any) {
	c.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, c.base+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if csrf {
		req.Header.Set("X-NovelCheck", "1")
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res, out
}

// doList is do() for endpoints returning a JSON array.
func (c *client) doList(method, path string) (*http.Response, []map[string]any) {
	c.t.Helper()
	req, _ := http.NewRequest(method, c.base+path, nil)
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	var out []map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res, out
}

func setup(t *testing.T) (*httptest.Server, *store.Store) { return setupWith(t, nil) }

// setupWith lets a test adjust the server (e.g. fake book lookups) first.
func setupWith(t *testing.T, adjust func(*api.Server)) (*httptest.Server, *store.Store) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	st := store.New(d)
	if err := auth.Bootstrap(st, "admin", "adminpass1"); err != nil {
		t.Fatal(err)
	}
	s := &api.Server{
		Cfg:     config.Config{CalibreDir: t.TempDir(), SessionDays: 1},
		Store:   st,
		Auth:    &auth.Manager{Store: st, SessionDays: 1},
		Worker:  analyzer.New(st),
		Syncer:  &calibre.Syncer{Store: st, Dir: t.TempDir()},
		Web:     fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}},
		Tunnel:  &tunnel.Manager{Binary: "no-such-cloudflared"},
		SysInfo: sysinfo.New(t.TempDir()),
		Push:    push.New(st),
	}
	if adjust != nil {
		adjust(s)
	}
	srv := httptest.NewServer(s.Router())
	t.Cleanup(srv.Close)
	return srv, st
}

func login(t *testing.T, srv *httptest.Server, user, pw string) *client {
	jar, _ := cookiejar.New(nil)
	c := &client{t: t, base: srv.URL, http: &http.Client{Jar: jar}}
	if res, _ := c.do("POST", "/api/auth/login", map[string]string{"username": user, "password": pw}, true); res.StatusCode != 200 {
		t.Fatalf("login %s: %d", user, res.StatusCode)
	}
	return c
}

func TestEndToEnd(t *testing.T) {
	srv, st := setup(t)
	anon := &client{t: t, base: srv.URL, http: http.DefaultClient}
	if res, _ := anon.do("GET", "/api/books", nil, false); res.StatusCode != 401 {
		t.Fatalf("anonymous access should be 401, got %d", res.StatusCode)
	}
	admin := login(t, srv, "admin", "adminpass1")

	// CSRF guard: state-changing call without the header is refused.
	if res, _ := admin.do("POST", "/api/admin/users", map[string]string{"username": "kid", "password": "kidpass12"}, false); res.StatusCode != 403 {
		t.Fatalf("expected CSRF rejection, got %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", "/api/admin/users", map[string]string{"username": "kid", "password": "kidpass12"}, true); res.StatusCode != 201 {
		t.Fatalf("create user: %d", res.StatusCode)
	}

	// Drive import into a new catalog.
	res, out := admin.do("POST", "/api/import/drive", map[string]any{
		"catalog_name": "Jenna's Kindle",
		"books": []map[string]string{
			{"title": "Clean Book", "author": "A. Writer", "format": "azw3", "path": "documents/Clean.azw3"},
			{"title": "Spicy Book", "author": "B. Writer", "format": "epub", "path": "documents/Spicy.epub"},
			{"title": "Ignored", "format": "txt"},
			// A pasted title list (Kindles that connect as a "device").
			{"title": "Listed Book", "author": "C. Writer", "format": "list", "path": "list:Listed Book | C. Writer"},
		},
	}, true)
	if res.StatusCode != 200 || out["imported"].(float64) != 3 || out["skipped"].(float64) != 1 {
		t.Fatalf("import: %d %v", res.StatusCode, out)
	}
	books, _, _ := st.ListBooks(store.BookFilter{}, nil)
	for _, b := range books {
		c := "No Spice"
		if b.Title == "Spicy Book" {
			c = "Open Door"
		}
		_ = st.SaveAnalysis(b.ID, store.Analysis{Classification: c})
	}

	kid := login(t, srv, "kid", "kidpass12")
	res, out = kid.do("GET", "/api/books", nil, false)
	if res.Header.Get("Cache-Control") != "no-store" || res.Header.Get("Strict-Transport-Security") == "" {
		t.Fatal("missing no-store / HSTS headers on API response")
	}
	if out["total"].(float64) != 2 { // Clean Book + Listed Book; Spicy hidden
		t.Fatalf("restricted user should see 2 books, got %v", out["total"])
	}
	if res, _ := kid.do("GET", "/api/admin/status", nil, false); res.StatusCode != 403 {
		t.Fatalf("restricted user reached admin API: %d", res.StatusCode)
	}
	var spicy int64
	for _, b := range books {
		if b.Title == "Spicy Book" {
			spicy = b.ID
		}
	}
	if res, _ := kid.do("POST", "/api/queue", map[string]int64{"book_id": spicy}, true); res.StatusCode != 404 {
		t.Fatalf("restricted user queued a hidden book: %d", res.StatusCode)
	}

	// Admin queue: add both, start reading with no delivery method.
	for _, b := range books {
		if res, _ := admin.do("POST", "/api/queue", map[string]int64{"book_id": b.ID}, true); res.StatusCode != 201 {
			t.Fatalf("enqueue: %d", res.StatusCode)
		}
	}
	items, _ := st.ListQueue(1, false)
	if res, out := admin.do("POST", "/api/queue/"+itoa(items[0].ID)+"/start", nil, true); res.StatusCode != 200 || out["status"] != "reading" {
		t.Fatalf("start reading: %d %v", res.StatusCode, out)
	}

	// SPA fallback serves index.html for client routes.
	r, err := http.Get(srv.URL + "/some/client/route")
	if err != nil || r.StatusCode != 200 {
		t.Fatalf("static fallback failed: %v %v", err, r)
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }
