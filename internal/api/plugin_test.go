package api_test

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/zachcurry13/novelcheck/internal/kosync"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The KOReader plugin lists Up Next and downloads a book, signed in like
// progress sync.
func TestKOReaderPlugin(t *testing.T) {
	lib := t.TempDir()
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.CalibreDir = lib })
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, code := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(code["code"].(string)))
	key := hex.EncodeToString(sum[:])
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	p := filepath.Join(lib, "dune.epub")
	_ = os.WriteFile(p, []byte("EPUB BYTES"), 0o644)
	dune, _ := st.UpsertBook("Dune", "Frank Herbert", "", "")
	_ = st.AddCopy(cal, dune, p, "epub", "1")
	nofile, _ := st.UpsertBook("No File", "Nobody", "", "")
	for _, id := range []int64{dune, nofile} {
		admin.do("POST", "/api/queue", map[string]int64{"book_id": id}, true)
	}
	get := func(path, k string) (*http.Response, string) {
		req, _ := http.NewRequest("GET", srv.URL+path, nil)
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", k)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res, string(b)
	}
	if res, _ := get("/kosync/plugin/up-next", "wrong"); res.StatusCode != 401 {
		t.Fatalf("wrong key: %d", res.StatusCode)
	}
	res, body := get("/kosync/plugin/up-next", key)
	if res.StatusCode != 200 || !strings.Contains(body, `"title":"Dune"`) || strings.Contains(body, "No File") || !strings.Contains(body, "/kosync/plugin/book/") {
		t.Fatalf("up next: %d %s", res.StatusCode, body)
	}
	if res, body := get("/kosync/plugin/book/"+fmt.Sprint(dune)+"/Dune.epub", key); res.StatusCode != 200 || body != "EPUB BYTES" {
		t.Fatalf("download: %d %q", res.StatusCode, body)
	}
	if res, _ := get("/kosync/plugin/book/"+fmt.Sprint(dune)+"/Dune.epub", "wrong"); res.StatusCode != 401 {
		t.Fatalf("download needs the key: %d", res.StatusCode)
	}

	// The device report: downloading Dune made its fingerprint known, so a
	// file with that fingerprint matches the book whatever it's called; a
	// file named after a library title matches by title; folders are cut off.
	post := func(body, k string) (*http.Response, string) {
		req, _ := http.NewRequest("POST", srv.URL+"/kosync/plugin/device", strings.NewReader(body))
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", k)
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res, string(b)
	}
	sumDune, err := kosync.PartialMD5(p)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100 && st.DocBook(sumDune) == 0; i++ { // the download remembers it in the background
		time.Sleep(20 * time.Millisecond)
	}
	report := `{"device": "Kindle Paperwhite", "books": [
	 {"md5": "` + sumDune + `", "filename": "weird-name.epub", "title": "Something Else", "format": "EPUB", "size": 10},
	 {"md5": "", "filename": "/mnt/us/documents/Dune.epub", "title": "Dune", "format": "epub", "size": 10},
	 {"md5": "ffff", "filename": "Unknown.epub", "title": "Not In The Library", "format": "epub", "size": 5}]}`
	if res, _ := post(report, "wrong"); res.StatusCode != 401 {
		t.Fatalf("report needs the key: %d", res.StatusCode)
	}
	if res, out := post(report, key); res.StatusCode != 200 || !strings.Contains(out, `"books":3`) || !strings.Contains(out, `"matched":2`) {
		t.Fatalf("report: %d %s", res.StatusCode, out)
	}
	if res, _ := post(`{"device": "", "books": []}`, key); res.StatusCode != 400 {
		t.Fatalf("a device needs a name: %d", res.StatusCode)
	}
	_, sync := admin.do("GET", "/api/me/reading-sync", nil, false)
	devs, _ := sync["devices"].([]any)
	if len(devs) != 1 || devs[0].(map[string]any)["name"] != "Kindle Paperwhite" || devs[0].(map[string]any)["books"].(float64) != 3 || devs[0].(map[string]any)["in_library"].(float64) != 2 {
		t.Fatalf("devices in Profile: %v", sync["devices"])
	}
}
