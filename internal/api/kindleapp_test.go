package api_test

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The Kindle app: a set-up download, books as AZW3 (the one Calibre wrote beside the EPUB counts), and the
// plugin's endpoints open to a page from the Kindle's own files.
func TestKindleApp(t *testing.T) {
	lib := t.TempDir()
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.CalibreDir = lib })
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, ks := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	key := hex.EncodeToString(sum[:])
	get := func(path string) (int, []byte, http.Header) {
		req, _ := http.NewRequest("GET", srv.URL+path, nil)
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", key)
		req.Header.Set("Origin", "file://")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, b, res.Header
	}

	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	mk := func(title string) int64 {
		id, _ := st.UpsertBook(title, "Jane Austen", "", "")
		path := filepath.Join(lib, title+".epub")
		_ = os.WriteFile(path, []byte("epub"), 0o644)
		_ = st.AddCopy(cal, id, path, "epub", "12")
		return id
	}
	ready, plain := mk("Emma"), mk("Persuasion")
	_ = os.WriteFile(filepath.Join(lib, "Emma.azw3"), []byte("azw3 file"), 0o644) // Calibre's conversion, not synced yet

	code, body, h := get("/kosync/plugin/kindle/" + itoa(ready) + "?check=1")
	if code != 200 || !strings.Contains(string(body), `"file":"Emma.azw3"`) || h.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("check: %d %s %v", code, body, h)
	}
	if code, body, _ := get("/kosync/plugin/kindle/" + itoa(ready)); code != 200 || string(body) != "azw3 file" {
		t.Fatalf("download: %d %q", code, body)
	}
	if code, body, _ := get("/kosync/plugin/kindle/" + itoa(plain)); code != 409 || !strings.Contains(string(body), "can't make one") {
		t.Fatalf("an EPUB only, with no Calibre Content server to convert it: %d %s", code, body)
	}

	req, _ := http.NewRequest("OPTIONS", srv.URL+"/kosync/plugin/search", nil)
	req.Header.Set("Origin", "file://")
	req.Header.Set("Access-Control-Request-Headers", "x-auth-user, x-auth-key")
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != 204 || !strings.Contains(res.Header.Get("Access-Control-Allow-Headers"), "x-auth-key") {
		t.Fatalf("preflight: %v %v", err, res)
	}
	if res, _ := http.Get(srv.URL + "/kosync/plugin/cover/" + itoa(ready)); res.StatusCode != 401 {
		t.Fatalf("covers need the sign-in: %d", res.StatusCode)
	}

	zres, err := admin.http.Get(srv.URL + "/api/me/kindle-app.zip")
	if err != nil || zres.StatusCode != 200 {
		t.Fatalf("zip: %v %v", err, zres)
	}
	raw, _ := io.ReadAll(zres.Body)
	zres.Body.Close()
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		got[f.Name] = string(b)
	}
	cfg := strings.TrimSuffix(strings.SplitN(got["extensions/novelcheck-app/config.js"], "var NC = ", 2)[len(strings.SplitN(got["extensions/novelcheck-app/config.js"], "var NC = ", 2))-1], ";\n")
	var nc map[string]string
	if err := json.Unmarshal([]byte(cfg), &nc); err != nil || nc["user"] != "admin" || nc["key"] != key || nc["server"] != srv.URL {
		t.Fatalf("config.js: %v %q", err, cfg)
	}
	if c := got["documents/NovelCheck-Check.sh"]; !strings.Contains(c, "NCKEY='"+key+"'") || !strings.Contains(c, "SERVER='"+srv.URL+"'") || strings.Contains(c, "{{") {
		t.Fatalf("the check script is filled in for this account: %q", c[:300])
	}
	if s := got["documents/NovelCheck.sh"]; !strings.Contains(s, "# Icon: data:image/png;base64,iVBOR") || got["extensions/novelcheck-app/app.js"] == "" {
		t.Fatalf("the launcher has its cover, the app its files: %v", len(got))
	}
}
