package api_test

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
}
