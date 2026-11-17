package api_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The cover finder asks Hardcover when Open Library and Google Books have nothing: a self-published book with
// a long subtitle in brackets is found under its short title, and Hardcover's cover is kept.
func TestCoverFinderAsksHardcover(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 60, 90))
	for x := 0; x < 60; x++ {
		for y := 0; y < 90; y++ {
			img.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 2), 120, 255})
		}
	}
	var jpg bytes.Buffer
	_ = jpeg.Encode(&jpg, img, nil)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/void.jpg" {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(jpg.Bytes())
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var q struct{ Query string }
		_ = json.Unmarshal(raw, &q)
		switch {
		case strings.Contains(q.Query, `search(query: "Descent into the Void Steve P. Vincent"`):
			io.WriteString(w, `{"data": {"search": {"ids": [44]}}}`)
		case strings.Contains(q.Query, "search("):
			io.WriteString(w, `{"data": {"search": {"ids": []}}}`)
		case strings.Contains(q.Query, "_in: [44]"):
			io.WriteString(w, `{"data": {"books": [{"id": 44, "title": "Descent into the Void", "cached_image": {"url": "https://assets.hardcover.app/void.jpg"},
				"contributions": [{"author": {"name": "Steve P. Vincent"}}]}]}}`)
		case q.Query != "":
			io.WriteString(w, `{"data": {"me": [{"id": 7, "username": "reader7"}]}}`)
		default: // Open Library and Google Books: nothing
			io.WriteString(w, `{"docs": [], "items": []}`)
		}
	}))
	defer fake.Close()
	// Hardcover's picture host is the fake server.
	to, _ := url.Parse(fake.URL)
	old := http.DefaultTransport
	http.DefaultTransport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "assets.hardcover.app" {
			r = r.Clone(r.Context())
			r.URL.Scheme, r.URL.Host = to.Scheme, to.Host
		}
		return old.RoundTrip(r)
	})
	t.Cleanup(func() { http.DefaultTransport = old })
	api.SetHardcoverURL(fake.URL)
	t.Cleanup(func() { api.SetHardcoverURL("") })
	srv, st := setupWith(t, func(s *api.Server) {
		s.Covers.OpenLibraryURL, s.Covers.SearchURL, s.Covers.GoogleURL = fake.URL, fake.URL, fake.URL
		s.Worker.NewEnricher = func() *enrich.Client {
			c := enrich.New("")
			c.OpenLibraryURL, c.GoogleBooksURL = fake.URL, fake.URL
			return c
		}
	})
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("PUT", "/api/me/hardcover", map[string]string{"token": "good-token"}, true)
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	b, _ := st.UpsertBook("Descent into the Void (An action packed science fiction adventure)", "Steve P. Vincent", "", "")
	_ = st.AddCopy(cal, b, "/c/none/void.epub", "epub", "1")

	if res, _ := admin.do("POST", "/api/admin/covers/find", map[string]string{}, true); res.StatusCode != 202 {
		t.Fatalf("start: %d", res.StatusCode)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		_, out := admin.do("GET", "/api/admin/covers/missing", nil, false)
		job, _ := out["job"].(map[string]any)
		if job["running"] != true && out["count"] == float64(0) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("still no cover: %v", out)
		}
		time.Sleep(200 * time.Millisecond)
	}
	if st.HCIDOf(b) != 44 {
		t.Fatalf("the Hardcover id is kept: %d", st.HCIDOf(b))
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
