package api_test

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/zachcurry13/novelcheck/internal/api"
)

// A phone must never run a mix of old and new app files: index.html points
// at this build's versioned addresses, which cache for good, and the plain
// addresses are revalidated (304 when unchanged).
func TestStaticFilesAreVersioned(t *testing.T) {
	srv, _ := setupWith(t, func(s *api.Server) {
		s.Web = fstest.MapFS{
			"index.html":      {Data: []byte(`<link rel="stylesheet" href="/css/app.css"><script src="/vendor/x.js"></script><script src="/js/app.js" type="module"></script><link rel="icon" href="/icons/icon.svg">`)},
			"js/app.js":       {Data: []byte(`import "./api.js";`)},
			"css/app.css":     {Data: []byte(`body{}`)},
			"vendor/x.js":     {Data: []byte(`x`)},
			"icons/icon.svg":  {Data: []byte(`<svg/>`)},
			"manifest.json":   {Data: []byte(`{}`)},
			"js/api.js":       {Data: []byte(`export {}`)},
			"sw.js":           {Data: []byte(`self`)},
			"css/unused.css":  {Data: []byte(``)},
			"icons/other.png": {Data: []byte(`png`)},
		}
	})
	get := func(path, etag string) (*http.Response, string) {
		req, _ := http.NewRequest("GET", srv.URL+path, nil)
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res, string(body)
	}

	res, index := get("/", "")
	m := regexp.MustCompile(`src="/v/([0-9a-f]+)/js/app.js"`).FindStringSubmatch(index)
	if res.StatusCode != 200 || m == nil || !strings.Contains(index, `href="/v/`+m[1]+`/css/app.css"`) ||
		!strings.Contains(index, `src="/v/`+m[1]+`/vendor/x.js"`) || !strings.Contains(index, `href="/icons/icon.svg"`) ||
		res.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("index: %d %q %q", res.StatusCode, res.Header.Get("Cache-Control"), index)
	}
	build := m[1]

	res, body := get("/v/"+build+"/js/app.js", "")
	if res.StatusCode != 200 || body != `import "./api.js";` || !strings.Contains(res.Header.Get("Cache-Control"), "immutable") {
		t.Fatalf("versioned: %d %q %q", res.StatusCode, res.Header.Get("Cache-Control"), body)
	}
	// Another build's address is served, but never cached for good.
	if res, _ = get("/v/0123456789ab/js/app.js", ""); res.StatusCode != 200 || res.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("old build: %d %q", res.StatusCode, res.Header.Get("Cache-Control"))
	}
	res, _ = get("/js/app.js", "")
	tag := res.Header.Get("ETag")
	if res.Header.Get("Cache-Control") != "no-cache" || tag == "" {
		t.Fatalf("plain: %q %q", res.Header.Get("Cache-Control"), tag)
	}
	if res, _ = get("/js/app.js", tag); res.StatusCode != http.StatusNotModified {
		t.Fatalf("revalidate: %d", res.StatusCode)
	}
	if res, _ = get("/icons/icon.svg", ""); res.Header.Get("Cache-Control") != "public, max-age=3600" {
		t.Fatalf("icon: %q", res.Header.Get("Cache-Control"))
	}
}
