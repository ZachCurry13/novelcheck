package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// A Calibre with one untidy book, answering the way the Content server does for `list` and `set_metadata`.
type fakeCalibreDetails struct {
	mu      sync.Mutex
	book    map[string]any // field -> value
	sets    [][]any
	uploads []string // add-book requests: path and size
}

func (f *fakeCalibreDetails) serve(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		raw, _ := io.ReadAll(r.Body)
		if strings.HasPrefix(r.URL.Path, "/cdb/add-book/") {
			f.uploads = append(f.uploads, r.URL.Path+" "+r.Header.Get("Content-Type")+" "+strconv.Itoa(len(raw)))
			io.WriteString(w, `{"book_id": 5, "title": "Cinder"}`)
			return
		}
		switch r.URL.Path {
		case "/ajax/library-info":
			io.WriteString(w, `{"library_map":{"books":"books"},"default_library":"books"}`)
		case "/cdb/cmd/list/0":
			data := map[string]map[string]any{}
			for field, v := range f.book {
				data[field] = map[string]any{"5": v}
			}
			b, _ := json.Marshal(map[string]any{"result": map[string]any{"data": data}})
			w.Write(b)
		case "/cdb/cmd/set_metadata/0":
			var args []json.RawMessage
			_ = json.Unmarshal(raw, &args)
			var fields [][]any
			_ = json.Unmarshal(args[2], &fields)
			f.sets = append(f.sets, fields...)
			for _, kv := range fields {
				name := kv[0].(string)
				if l, ok := kv[1].([]any); ok && name == "authors" {
					var names []string
					for _, a := range l {
						names = append(names, a.(string))
					}
					f.book[name] = strings.Join(names, " & ")
					continue
				}
				f.book[name] = kv[1]
			}
			io.WriteString(w, `{"result":{"!_":2,"!v":{}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func cleanupHardcover(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var q struct{ Query string }
		_ = json.Unmarshal(raw, &q)
		switch {
		case strings.Contains(q.Query, "isbn_13: {_in:"):
			io.WriteString(w, `{"data": {"editions": [{"isbn_13": "9780312641894", "isbn_10": "", "pages": 400, "book": {"id": 9, "title": "Cinder",
				"release_year": 2012, "description": "Humanity needs a cyborg mechanic.", "cached_image": {"url": "https://example.test/c.jpg"},
				"cached_tags": {"Genre": [{"tag": "Science Fiction"}, {"tag": "Young Adult"}]},
				"contributions": [{"author": {"name": "Marissa Meyer"}}],
				"book_series": [{"position": 1, "series": {"name": "The Lunar Chronicles"}}]}}]}}`)
		case strings.Contains(q.Query, "_in: [1, 2, 3, 4]"):
			io.WriteString(w, `{"data": {"me": [{"user_books": []}]}}`)
		case strings.Contains(q.Query, "goals("):
			io.WriteString(w, `{"data": {"me": [{"goals": []}]}}`)
		default:
			io.WriteString(w, `{"data": {"me": [{"id": 7, "username": "reader7"}]}}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func waitHC(t *testing.T, admin *client) map[string]any {
	t.Helper()
	for i := 0; i < 200; i++ {
		_, st := admin.do("GET", "/api/admin/hc-clean", nil, false)
		if job, _ := st["job"].(map[string]any); job["running"] == false {
			return st
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the Hardcover job never finished")
	return nil
}

func TestHardcoverCleanup(t *testing.T) {
	hc := cleanupHardcover(t)
	api.SetHardcoverURL(hc.URL)
	t.Cleanup(func() { api.SetHardcoverURL("") })
	cal := &fakeCalibreDetails{book: map[string]any{
		"title": "Cinder (The Lunar Chronicles Book 1)", "authors": "Meyer, Marissa", "series": "", "series_index": 1.0,
		"tags": []string{"Young Adult"}, "comments": "", "identifiers": map[string]string{}, "cover": "/c/cover.jpg",
	}}
	cs := cal.serve(t)
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	catalog, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	id, _ := st.UpsertBook("Cinder", "Marissa Meyer", "9780312641894", "")
	_ = st.AddCopy(catalog, id, "/c/cinder.epub", "epub", "5")

	if res, _ := admin.do("POST", "/api/admin/hc-clean/scan", nil, true); res.StatusCode != 400 {
		t.Fatalf("a scan needs Hardcover and the Content server first: %d", res.StatusCode)
	}
	admin.do("PUT", "/api/me/hardcover", map[string]string{"token": "good-token"}, true)
	admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": cs.URL}, true)
	if res, out := admin.do("POST", "/api/admin/hc-clean/scan", nil, true); res.StatusCode != 202 {
		t.Fatalf("scan: %d %v", res.StatusCode, out)
	}
	state := waitHC(t, admin)
	var fields []string
	var ids []int64
	for _, s := range state["suggestions"].([]any) {
		m := s.(map[string]any)
		fields = append(fields, m["field"].(string))
		ids = append(ids, int64(m["id"].(float64)))
	}
	got := strings.Join(fields, ",")
	for _, want := range []string{"title", "authors", "series", "series_index", "identifiers", "comments", "tags"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing a %s suggestion in %s", want, got)
		}
	}
	if len(cal.sets) != 0 {
		t.Fatalf("a scan never writes to Calibre: %v", cal.sets)
	}

	// Skip the description; apply the rest.
	var commentID int64
	for _, s := range state["suggestions"].([]any) {
		if m := s.(map[string]any); m["field"] == "comments" {
			commentID = int64(m["id"].(float64))
		}
	}
	admin.do("POST", "/api/admin/hc-clean/skip", map[string]any{"ids": []int64{commentID}}, true)
	var rest []int64
	for _, i := range ids {
		if i != commentID {
			rest = append(rest, i)
		}
	}
	if res, out := admin.do("POST", "/api/admin/hc-clean/apply", map[string]any{"ids": rest}, true); res.StatusCode != 202 {
		t.Fatalf("apply: %d %v", res.StatusCode, out)
	}
	state = waitHC(t, admin)
	job := state["job"].(map[string]any)
	if job["found"].(float64) != 1 || len(job["errors"].([]any)) != 0 {
		t.Fatalf("one book fixed with nothing refused: %v", job)
	}
	if cal.book["title"] != "Cinder" || cal.book["series"] != "The Lunar Chronicles" || cal.book["authors"] != "Marissa Meyer" || cal.book["comments"] != "" {
		t.Fatalf("Calibre now has: %v", cal.book)
	}
	if n := len(state["suggestions"].([]any)); n != 0 {
		t.Fatalf("applied and skipped suggestions leave the list: %d", n)
	}

	// Scanning again does not bring back what was skipped or applied.
	admin.do("POST", "/api/admin/hc-clean/scan", nil, true)
	state = waitHC(t, admin)
	if n := len(state["suggestions"].([]any)); n != 0 {
		t.Fatalf("nothing left to suggest: %v", state["suggestions"])
	}
}
