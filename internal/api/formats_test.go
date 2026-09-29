package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// fakeLibrary mimics calibre's Content server for format cleanup: removed
// formats go to <lib>/.caltrash/f/<id>/<fmt>, add_format takes msgpack, and a
// conversion adds the EPUB when its finished status is read.
type fakeLibrary struct {
	mu      sync.Mutex
	lib     string
	titles  map[string]string
	formats map[string][]string
	added   int
}

func (f *fakeLibrary) has(id, format string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, x := range f.formats[id] {
		if x == format {
			return true
		}
	}
	return false
}

func (f *fakeLibrary) serve(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		switch {
		case r.URL.Path == "/ajax/library-info":
			fmt.Fprint(w, `{"library_map":{"books":"books"},"default_library":"books"}`)
		case r.URL.Path == "/cdb/cmd/list/0":
			b, _ := json.Marshal(map[string]any{"result": map[string]any{"data": map[string]any{"title": f.titles, "formats": f.formats}}})
			w.Write(b)
		case r.URL.Path == "/cdb/cmd/remove_format/0":
			var args []any
			_ = json.Unmarshal(body, &args)
			id, format := fmt.Sprint(args[0]), args[1].(string)
			var left []string
			for _, x := range f.formats[id] {
				if x != format {
					left = append(left, x)
				}
			}
			f.formats[id] = left
			dir := filepath.Join(f.lib, ".caltrash", "f", id)
			_ = os.MkdirAll(dir, 0o755)
			_ = os.WriteFile(filepath.Join(dir, strings.ToLower(format)), []byte("file of "+format), 0o644)
			fmt.Fprint(w, `{"result":null}`)
		case r.URL.Path == "/cdb/cmd/add_format/0":
			if r.Header.Get("Content-Type") != "application/x-msgpack" || !bytes.Contains(body, []byte("file of ")) {
				t.Errorf("add_format must carry the file as msgpack: %q", r.Header.Get("Content-Type"))
			}
			// [id, [name, bytes], FMT, false]: the format is the last string before false.
			fmtName := string(body[len(body)-5 : len(body)-1])
			id := fmt.Sprint(int(body[1]))
			f.formats[id] = append(f.formats[id], strings.TrimLeft(fmtName, "\xa3\xa4"))
			f.added++
			fmt.Fprint(w, `{"result":true}`)
		case r.URL.Path == "/conversion/start/2":
			fmt.Fprint(w, `5`)
		case r.URL.Path == "/conversion/status/5":
			f.formats["2"] = append(f.formats["2"], "EPUB")
			fmt.Fprint(w, `{"running":false,"ok":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func waitFormatJob(t *testing.T, admin *client) map[string]any {
	t.Helper()
	for i := 0; i < 100; i++ {
		_, st := admin.do("GET", "/api/admin/formats/job", nil, false)
		if st["running"] == false {
			return st
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("format job never finished")
	return nil
}

func TestFormatCleanup(t *testing.T) {
	lib := t.TempDir()
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.CalibreDir, s.Syncer.Dir = lib, lib })
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	files := map[string][]string{"1": {"EPUB", "MOBI", "AZW3"}, "2": {"MOBI"}, "3": {"PDF", "MOBI"}}
	for i, title := range []string{"Alpha", "Beta", "Gamma"} {
		id, _ := st.UpsertBook(title, "Author", "", "")
		cid := fmt.Sprint(i + 1)
		for _, f := range files[cid] {
			p := filepath.Join(lib, title, title+"."+strings.ToLower(f))
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			_ = os.WriteFile(p, bytes.Repeat([]byte("x"), 1000), 0o644)
			_ = st.AddCopy(cal, id, p, strings.ToLower(f), cid)
		}
	}
	fake := &fakeLibrary{lib: lib, titles: map[string]string{"1": "Alpha", "2": "Beta", "3": "Gamma"}, formats: files}
	calibre := fake.serve(t)

	// Keeping EPUB: only Alpha has one, so only its MOBI and AZW3 would go.
	_, prev := admin.do("GET", "/api/admin/formats?keep=EPUB", nil, false)
	if prev["group_count"].(float64) != 1 || prev["file_count"].(float64) != 2 || prev["bytes"].(float64) != 2000 || prev["convert_count"].(float64) != 2 {
		t.Fatalf("preview: %v", prev)
	}
	remove := map[string]any{"keep": []string{"EPUB"}, "expected_files": 2}
	if res, _ := admin.do("POST", "/api/admin/formats/remove", remove, true); res.StatusCode != 400 {
		t.Fatalf("without a Content server it must be refused: %d", res.StatusCode)
	}
	admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": calibre.URL}, true)
	if res, _ := admin.do("POST", "/api/admin/formats/remove", map[string]any{"keep": []string{"EPUB"}, "expected_files": 3}, true); res.StatusCode != 409 {
		t.Fatalf("a stale count must be refused: %d", res.StatusCode)
	}
	if res, out := admin.do("POST", "/api/admin/formats/remove", remove, true); res.StatusCode != 202 {
		t.Fatalf("remove: %d %v", res.StatusCode, out)
	}
	if job := waitFormatJob(t, admin); job["done"].(float64) != 2 || !strings.Contains(job["note"].(string), "Removed 2 files") {
		t.Fatalf("removal job: %v", job)
	}
	if fake.has("1", "MOBI") || !fake.has("1", "EPUB") || !fake.has("2", "MOBI") || !fake.has("3", "PDF") {
		t.Fatalf("wrong formats removed: %v", fake.formats)
	}

	// Undo the whole run from calibre's recycle bin.
	_, prev = admin.do("GET", "/api/admin/formats", nil, false)
	batches := prev["batches"].([]any)
	if len(batches) != 1 || batches[0].(map[string]any)["files"].(float64) != 2 {
		t.Fatalf("removals to undo: %v", prev["batches"])
	}
	batch := batches[0].(map[string]any)["batch"]
	if res, out := admin.do("POST", "/api/admin/formats/undo", map[string]any{"batch": batch}, true); res.StatusCode != 202 {
		t.Fatalf("undo: %d %v", res.StatusCode, out)
	}
	if job := waitFormatJob(t, admin); job["done"].(float64) != 2 || fake.added != 2 || !fake.has("1", "MOBI") || !fake.has("1", "AZW3") {
		t.Fatalf("undo job: %v formats %v", job, fake.formats)
	}
	if _, prev = admin.do("GET", "/api/admin/formats", nil, false); len(prev["batches"].([]any)) != 0 {
		t.Fatalf("a restored run should leave the Undo list: %v", prev["batches"])
	}

	// Converting checks titles first: the wrong library converts nothing.
	fake.mu.Lock()
	fake.titles["2"] = "Grandma's Cookbook"
	fake.mu.Unlock()
	beta, _ := st.UpsertBook("Beta", "Author", "", "")
	admin.do("POST", "/api/admin/formats/convert", map[string]any{"book_ids": []int64{beta}}, true)
	if job := waitFormatJob(t, admin); !strings.Contains(job["note"].(string), "Stopped") || fake.has("2", "EPUB") {
		t.Fatalf("a title mismatch must stop the conversion: %v", job)
	}
	fake.mu.Lock()
	fake.titles["2"] = "Beta"
	fake.mu.Unlock()
	admin.do("POST", "/api/admin/formats/convert", map[string]any{"book_ids": []int64{beta}}, true)
	if job := waitFormatJob(t, admin); job["done"].(float64) != 1 || !fake.has("2", "EPUB") {
		t.Fatalf("conversion: %v", job)
	}
}
