package api_test

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// A MOBI-shaped file: the header says where record 0 is, and record 0 holds the encryption type.
func fakeMobi(enc uint16) []byte {
	b := make([]byte, 4096)
	binary.BigEndian.PutUint32(b[78:], 100)
	binary.BigEndian.PutUint16(b[100+12:], enc)
	return b
}

// "Copy my Kindle": every book file is kept as it is (protected ones too), things that only look like books are
// not asked for, and what is already on the server is not sent again.
func TestKindleCopy(t *testing.T) {
	dir := t.TempDir()
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.DataDir = dir })
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, ks := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	key := hex.EncodeToString(sum[:])
	call := func(method, path string, body []byte) map[string]any {
		req, _ := http.NewRequest(method, srv.URL+"/kosync/plugin/kindle/"+path, bytes.NewReader(body))
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", key)
		res, err := http.DefaultClient.Do(req)
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("%s %s: %v %v", method, path, err, res)
		}
		var out map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		return out
	}
	check := func() map[string]any {
		b, _ := json.Marshal(map[string]any{"device": "Test Kindle", "files": []map[string]any{
			{"path": "/mnt/us/documents/Mine.azw3", "size": 4096},
			{"path": "/mnt/us/documents/Store Book.azw", "size": 4096},
			{"path": "/mnt/us/documents/novelcheck-bug-log.azw3", "size": 9000},
			{"path": "/mnt/us/koreader/settings.epub", "size": 9000},
			{"path": "/mnt/us/Books/NovelCheck/The-Hobbit.epub", "size": 9000},
			{"path": "/mnt/us/documents/notes.txt", "size": 9000},
		}})
		return call("POST", "check", b)
	}
	out := check()
	send, _ := out["send"].([]any)
	if len(send) != 2 {
		t.Fatalf("two books are asked for, the log, KOReader's folder and the text file are not: %v", out)
	}
	if o := call("PUT", "file?device=Test+Kindle&path=/mnt/us/documents/Mine.azw3", fakeMobi(0)); o["locked"] != false {
		t.Fatalf("a plain MOBI is not locked: %v", o)
	}
	if o := call("PUT", "file?device=Test+Kindle&path=/mnt/us/documents/Store%20Book.azw", fakeMobi(2)); o["locked"] != true {
		t.Fatalf("a Kindle store book is kept, as locked: %v", o)
	}
	if _, err := os.Stat(filepath.Join(dir, "kindle-backup", "1", "Test Kindle", "documents", "Store Book.azw")); err != nil {
		t.Fatalf("the locked book is on disk: %v", err)
	}
	if out := check(); len(out["send"].([]any)) != 0 || out["have"] != 2.0 {
		t.Fatalf("nothing is sent twice: %v", out)
	}
	_, kb := admin.do("GET", "/api/admin/kindle-backup", nil, false)
	if devs, _ := kb["devices"].([]any); len(devs) != 1 || devs[0].(map[string]any)["files"] != 2.0 {
		t.Fatalf("the copied books are listed: %v", kb)
	}
	_, lf := admin.do("GET", "/api/admin/kindle-backup/files?user=1&device=Test+Kindle", nil, false)
	files, _ := lf["files"].([]any)
	if len(files) != 2 {
		t.Fatalf("the files are listed: %v", lf)
	}
	locked := 0
	for _, f := range files {
		if f.(map[string]any)["locked"] == true {
			locked++
		}
	}
	if locked != 1 {
		t.Fatalf("the store book is marked locked: %v", lf)
	}
	for _, u := range []string{"/api/admin/kindle-backup/file?user=1&device=Test+Kindle&path=documents/Store%20Book.azw", "/api/admin/kindle-backup/zip?user=1&device=Test+Kindle"} {
		res, err := admin.http.Get(srv.URL + u)
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("download %s: %v %v", u, err, res)
		}
	}
	if res, _ := admin.do("GET", "/api/admin/kindle-backup/file?user=1&device=Test+Kindle&path=../../novelcheck.db", nil, false); res.StatusCode != 404 {
		t.Fatalf("no way out of the folder: %d", res.StatusCode)
	}
}

// "Back up my Kindle": every file of the user storage except hidden and system folders and caches; a file is sent
// again when its change time differs (settings change without changing size); a small file counts.
func TestKindleBackupKeepsSettingsAndPlugins(t *testing.T) {
	dir := t.TempDir()
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.DataDir = dir })
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, ks := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	key := hex.EncodeToString(sum[:])
	call := func(method, path string, body []byte) map[string]any {
		req, _ := http.NewRequest(method, srv.URL+"/kosync/plugin/kindle/"+path, bytes.NewReader(body))
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", key)
		res, err := http.DefaultClient.Do(req)
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("%s %s: %v %v", method, path, err, res)
		}
		var out map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		return out
	}
	check := func(mtime int64) map[string]any {
		b, _ := json.Marshal(map[string]any{"device": "Test Kindle", "mode": "backup", "files": []map[string]any{
			{"path": "/mnt/us/koreader/settings/reader.lua", "size": 120, "mtime": mtime},
			{"path": "/mnt/us/koreader/plugins/novelcheck.koplugin/main.lua", "size": 500, "mtime": 5},
			{"path": "/mnt/us/extensions/kual/menu.json", "size": 80, "mtime": 5},
			{"path": "/mnt/us/koreader/cache/thumb.png", "size": 900, "mtime": 5},
			{"path": "/mnt/us/.active_content_sandbox/x.db", "size": 900, "mtime": 5},
			{"path": "/mnt/us/system/version.txt", "size": 20, "mtime": 5},
		}})
		return call("POST", "check", b)
	}
	out := check(100)
	if send, _ := out["send"].([]any); len(send) != 3 {
		t.Fatalf("three files are asked for, not the cache, the hidden folder or system: %v", out)
	}
	call("PUT", "file?device=Test+Kindle&mode=backup&mtime=100&path=/mnt/us/koreader/settings/reader.lua", bytes.Repeat([]byte("x"), 120))
	out = check(100)
	if send, _ := out["send"].([]any); len(send) != 2 || out["have"] != 1.0 {
		t.Fatalf("a saved file is not sent again: %v", out)
	}
	out = check(200)
	if send, _ := out["send"].([]any); len(send) != 3 {
		t.Fatalf("a file changed since is sent again: %v", out)
	}
}
