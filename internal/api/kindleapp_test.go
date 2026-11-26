package api_test

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
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

	if code, body, _ := get("/kosync/plugin/kindle-app"); code != 200 || !strings.Contains(string(body), `"update.js"`) || strings.Contains(string(body), "config.js") {
		t.Fatalf("the app's version and files, for updating: %d %s", code, body)
	}
	if code, _, _ := get("/kosync/plugin/kindle-app?v=1.5.0&device=Zach%20Paperwhite%20(Amazon%20reader)"); code != 200 {
		t.Fatalf("the update check: %d", code)
	}
	if v := st.Setting("app_seen_1"); !strings.Contains(v, "Zach Paperwhite (Amazon reader)") || !strings.Contains(v, "app 1.5.0") {
		t.Fatalf("noted for the bug link: %q", v)
	}
	if code, body, _ := get("/kosync/plugin/kindle-app/app.js"); code != 200 || !strings.Contains(string(body), "ES5") {
		t.Fatalf("an app file: %d", code)
	}
	if code, body, _ := get("/kosync/plugin/kindle-app/config.js?server=https://nc.example&name=Sis"); code != 200 || !strings.Contains(string(body), `"server":"https://nc.example"`) || !strings.Contains(string(body), `"device":"Sis"`) {
		t.Fatalf("the signed-in account's own config, for the plugin's install: %d %s", code, body)
	}
	if code, body, _ := get("/kosync/plugin/kindle-app/NovelCheck.sh"); code != 200 || !strings.HasPrefix(string(body), "#!/bin/sh") || strings.Contains(string(body), "{{ICON}}") {
		t.Fatalf("the launcher: %d", code)
	}
	// the app installed from KindleForge signs in once with the password and gets the same key
	post := func(pw string) (int, string) {
		res, err := http.Post(srv.URL+"/kosync/plugin/app-signin", "application/json", strings.NewReader(`{"username":"admin","password":"`+pw+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	if c, b := post("adminpass1"); c != 200 || !strings.Contains(b, key) || strings.Contains(b, ks["code"].(string)) {
		t.Fatalf("app sign-in: %d %s", c, b)
	}
	if c, _ := post("wrong"); c != 401 {
		t.Fatalf("a wrong password: %d", c)
	}
	// the plugin's sign-in file, for the Kindle app to install the KOReader plugin already connected
	if code, body, _ := get("/kosync/plugin/config.lua?server=https://nc.example"); code != 200 || !strings.Contains(string(body), `user = "admin"`) ||
		!strings.Contains(string(body), `code = "`+ks["code"].(string)+`"`) || !strings.Contains(string(body), `server = "https://nc.example"`) {
		t.Fatalf("plugin config: %d %s", code, body)
	}
	if res, err := http.Get(srv.URL + "/kosync/plugin/config.lua"); err != nil || res.StatusCode != 401 {
		t.Fatalf("the plugin's sign-in needs a sign-in: %v %v", err, res)
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

	_, wife := admin.do("POST", "/api/admin/users", map[string]string{"username": "partner", "password": "partnerpass1", "role": "admin"}, true)
	pres, err := admin.http.Get(srv.URL + "/api/admin/users/" + fmt.Sprint(wife["id"]) + "/kindle-app.zip")
	if err != nil || pres.StatusCode != 200 {
		t.Fatalf("the main admin downloads the app for the other parent: %v %v", err, pres)
	}
	praw, _ := io.ReadAll(pres.Body)
	pres.Body.Close()
	if !strings.Contains(unzip(t, praw)["extensions/novelcheck-app/config.js"], `"user":"partner"`) {
		t.Fatal("that download signs in as the other parent")
	}
	zres, err := admin.http.Get(srv.URL + "/api/me/kindle-app.zip")
	if err != nil || zres.StatusCode != 200 {
		t.Fatalf("zip: %v %v", err, zres)
	}
	raw, _ := io.ReadAll(zres.Body)
	zres.Body.Close()
	got := unzip(t, raw)
	cfg := strings.TrimSuffix(strings.SplitN(got["extensions/novelcheck-app/config.js"], "var NC = ", 2)[len(strings.SplitN(got["extensions/novelcheck-app/config.js"], "var NC = ", 2))-1], ";\n")
	var nc map[string]string
	if err := json.Unmarshal([]byte(cfg), &nc); err != nil || nc["user"] != "admin" || nc["key"] != key || nc["server"] != srv.URL {
		t.Fatalf("config.js: %v %q", err, cfg)
	}
	for name := range got {
		if strings.HasPrefix(name, "documents/") && name != "documents/NovelCheck.sh" {
			t.Fatalf("only the app's own book goes in documents (anything else shows in the library): %s", name)
		}
	}
	if !strings.Contains(got["extensions/novelcheck-app/check.sh"], "config.js") {
		t.Fatal("the check comes with the app and reads its account from config.js")
	}
	if s := got["documents/NovelCheck.sh"]; !strings.Contains(s, "# Icon: data:image/png;base64,iVBOR") || got["extensions/novelcheck-app/app.js"] == "" {
		t.Fatalf("the launcher has its cover, the app its files: %v", len(got))
	}
}

// unzip is a zip's files by name.
func unzip(t *testing.T, raw []byte) map[string]string {
	t.Helper()
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
	return got
}
