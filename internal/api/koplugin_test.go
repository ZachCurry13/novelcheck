package api_test

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
	kindleplugin "github.com/zachcurry13/novelcheck/kindle-plugin"
)

// NovelCheck hands out the KOReader plugin already connected to the person's
// account, and notes which plugin version each device runs.
func TestKOReaderPluginDownload(t *testing.T) {
	srv, st := setup(t)
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, ks := admin.do("GET", "/api/me/kosync", nil, false)
	code := ks["code"].(string)

	if res, err := http.Get(srv.URL + "/api/me/koplugin.zip"); err != nil || res.StatusCode != 401 {
		t.Fatalf("anonymous download: %v", err)
	}
	res, err := admin.http.Get(srv.URL + "/api/me/koplugin.zip")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "application/zip" || res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("download: %d %v", res.StatusCode, res.Header)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		r, _ := f.Open()
		b, _ := io.ReadAll(r)
		r.Close()
		files[f.Name] = string(b)
	}
	cfg := files["novelcheck.koplugin/config.lua"]
	if !strings.Contains(cfg, `user = "admin"`) || !strings.Contains(cfg, `code = "`+code+`"`) || !strings.Contains(cfg, "server = ") {
		t.Fatalf("config.lua: %q", cfg)
	}
	for _, name := range []string{"_meta.lua", "main.lua", "devicebooks.lua"} {
		if files["novelcheck.koplugin/"+name] == "" {
			t.Fatalf("%s missing from the zip", name)
		}
	}
	// The version in the Lua code and in _meta.lua are the one NovelCheck reports.
	v := kindleplugin.Version()
	if v == "" || !strings.Contains(files["novelcheck.koplugin/main.lua"], `local VERSION = "`+v+`"`) {
		t.Fatalf("main.lua version does not match _meta.lua (%q)", v)
	}

	// A device reports its plugin version; Profile shows it and the latest.
	sum := md5.Sum([]byte(code))
	body := `{"device":"Test Kindle","plugin_version":"1.0.0","books":[{"filename":"a.epub","title":"A","format":"epub","size":1}]}`
	req, _ := http.NewRequest("POST", srv.URL+"/kosync/plugin/device", strings.NewReader(body))
	req.Header.Set("x-auth-user", "admin")
	req.Header.Set("x-auth-key", hex.EncodeToString(sum[:]))
	if res, err := http.DefaultClient.Do(req); err != nil || res.StatusCode != 200 {
		t.Fatalf("report: %v", err)
	}
	_, rs := admin.do("GET", "/api/me/reading-sync", nil, false)
	devs, _ := rs["devices"].([]any)
	if len(devs) != 1 || devs[0].(map[string]any)["plugin_version"] != "1.0.0" || rs["plugin_latest"] != v {
		t.Fatalf("devices: %v latest %v", rs["devices"], rs["plugin_latest"])
	}
}
