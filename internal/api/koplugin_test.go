package api_test

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
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
	for _, name := range []string{"_meta.lua", "main.lua", "devicebooks.lua", "updater.lua", "devicereport.lua"} {
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

// Books on the e-reader go into Up Next by themselves, and a hidden one stays hidden.
func TestKindleBooksJoinUpNext(t *testing.T) {
	srv, st := setup(t)
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, ks := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	key := hex.EncodeToString(sum[:])
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	mk := func(title string) int64 {
		id, _ := st.UpsertBook(title, "Author", "", "")
		_ = st.AddCopy(cal, id, "/c/"+title+".epub", "epub", "1")
		return id
	}
	mk("Fresh Book")
	started, done := mk("Started Book"), mk("Done Book")
	mk("Hide Me")
	_ = st.SaveKosync(1, store.KosyncProgress{Document: "doc-started", Percentage: 0.4, Device: "Kindle", Timestamp: 100})
	st.SetDocBook("doc-started", started)
	_ = st.SaveKosync(1, store.KosyncProgress{Document: "doc-done", Percentage: 1, Device: "Kindle", Timestamp: 100})
	st.SetDocBook("doc-done", done)

	report := func(titles ...string) map[string]any {
		var books []string
		for _, ti := range titles {
			books = append(books, `{"filename":"`+ti+`.epub","title":"`+ti+`","format":"epub","size":1}`)
		}
		req, _ := http.NewRequest("POST", srv.URL+"/kosync/plugin/device",
			strings.NewReader(`{"device":"Kindle","plugin_version":"1.39.0","books":[`+strings.Join(books, ",")+`]}`))
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", key)
		res, err := http.DefaultClient.Do(req)
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("report: %v", err)
		}
		out := map[string]any{}
		_ = json.NewDecoder(res.Body).Decode(&out)
		res.Body.Close()
		return out
	}
	statusOf := func() map[string]string {
		_, items := admin.doList("GET", "/api/queue")
		m := map[string]string{}
		for _, it := range items {
			m[it["title"].(string)] = it["status"].(string)
		}
		return m
	}
	if out := report("Fresh Book", "Started Book", "Done Book", "Hide Me", "Unknown Book"); out["added"] != float64(3) {
		t.Fatalf("report: %v", out)
	}
	got := statusOf()
	if len(got) != 3 || got["Fresh Book"] != "queued" || got["Started Book"] != "reading" || got["Hide Me"] != "queued" {
		t.Fatalf("up next after the report (finished and unknown books stay out): %v", got)
	}

	// Hiding takes a book off the list, and the next report leaves it hidden.
	_, items := admin.doList("GET", "/api/queue")
	var hideItem float64
	for _, it := range items {
		if it["title"] == "Hide Me" {
			hideItem = it["id"].(float64)
		}
	}
	if res, _ := admin.do("POST", "/api/queue/"+itoa(int64(hideItem))+"/hide", map[string]bool{}, true); res.StatusCode != 200 {
		t.Fatalf("hide: %d", res.StatusCode)
	}
	if out := report("Fresh Book", "Started Book", "Done Book", "Hide Me"); out["added"] != float64(0) {
		t.Fatalf("second report: %v", out)
	}
	if got := statusOf(); len(got) != 2 || got["Hide Me"] != "" {
		t.Fatalf("hidden book came back: %v", got)
	}
	if _, hid := admin.doList("GET", "/api/queue?hidden=1"); len(hid) != 1 || hid[0]["title"] != "Hide Me" {
		t.Fatalf("hidden list: %v", hid)
	}
	admin.do("POST", "/api/queue/"+itoa(int64(hideItem))+"/hide", map[string]bool{"hidden": false}, true)
	if got := statusOf(); got["Hide Me"] != "queued" {
		t.Fatalf("unhide: %v", got)
	}

	// With the switch off, a report adds nothing.
	admin.do("DELETE", "/api/queue/"+itoa(int64(hideItem)), nil, true)
	admin.do("PUT", "/api/me/kindle-autoadd", map[string]bool{"enabled": false}, true)
	if out := report("Hide Me"); out["added"] != float64(0) {
		t.Fatalf("switched off: %v", out)
	}
}

// The plugin can ask for the latest version and fetch its own files to update.
func TestKOReaderPluginUpdate(t *testing.T) {
	srv, st := setup(t)
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, ks := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	get := func(path, key string) (int, string) {
		req, _ := http.NewRequest("GET", srv.URL+path, nil)
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", key)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return res.StatusCode, string(b)
	}
	key := hex.EncodeToString(sum[:])
	if code, _ := get("/kosync/plugin/version", "wrong"); code != 401 {
		t.Fatalf("signed out: %d", code)
	}
	code, body := get("/kosync/plugin/version", key)
	v := kindleplugin.Version()
	if code != 200 || !strings.Contains(body, `"version":"`+v+`"`) || !strings.Contains(body, "main.lua") || !strings.Contains(body, "devicebooks.lua") || strings.Contains(body, "config.lua") {
		t.Fatalf("version: %d %s", code, body)
	}
	if code, lua := get("/kosync/plugin/file/main.lua", key); code != 200 || !strings.Contains(lua, `local VERSION = "`+v+`"`) {
		t.Fatalf("main.lua: %d", code)
	}
	for _, bad := range []string{"config.lua", "..%2Fmain.lua", "nothing.lua"} {
		if code, _ := get("/kosync/plugin/file/"+bad, key); code != 404 {
			t.Fatalf("%s: %d", bad, code)
		}
	}
}
