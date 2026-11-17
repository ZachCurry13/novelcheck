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
	// books NovelCheck does not know are noted for review, not made up and not put on the Nightstand
	call("PUT", "file?device=Test+Kindle&path=/mnt/us/documents/B07ABCDEF1_EBOK.azw", fakeMobi(2))
	if _, items := admin.doList("GET", "/api/queue"); len(items) != 0 {
		t.Fatalf("nothing unknown goes on the Nightstand by itself: %v", items)
	}
	_, unk := admin.do("GET", "/api/admin/kindle-unknown", nil, false)
	if bks, _ := unk["books"].([]any); len(bks) != 2 {
		t.Fatalf("the two unknown books wait for review, the store-code file is not one: %v", unk)
	}
	// a book NovelCheck has goes on the Nightstand, by "Title - Author" or "Author - Title" alike
	id, _ := st.UpsertBook("Mine", "Some Author", "", "")
	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	_ = st.AddCopy(cat, id, "/c/mine.epub", "epub", "9")
	call("PUT", "file?device=Test+Kindle&path=/mnt/us/documents/Mine%20-%20Some%20Author.azw3", fakeMobi(0))
	if _, items := admin.doList("GET", "/api/queue"); len(items) != 1 {
		t.Fatalf("a known book goes on the Nightstand: %v", items)
	}
	// ticking an unknown one adds it and puts it on the Nightstand
	var first float64
	for _, b := range unk["books"].([]any) {
		first = b.(map[string]any)["id"].(float64)
		break
	}
	if res, out := admin.do("POST", "/api/admin/kindle-unknown", map[string]any{"add": []int64{int64(first)}}, true); res.StatusCode != 200 || out["added"] != 1.0 {
		t.Fatalf("add the ticked one: %d %v", res.StatusCode, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "kindle-backup", "1", "Test Kindle", "documents", "Store Book.azw")); err != nil {
		t.Fatalf("the locked book is on disk: %v", err)
	}
	if out := check(); len(out["send"].([]any)) != 0 || out["have"] != 2.0 {
		t.Fatalf("nothing is sent twice: %v", out)
	}
	_, kb := admin.do("GET", "/api/admin/kindle-backup", nil, false)
	if devs, _ := kb["devices"].([]any); len(devs) != 1 || devs[0].(map[string]any)["files"] != 4.0 {
		t.Fatalf("the copied books are listed: %v", kb)
	}
	_, lf := admin.do("GET", "/api/admin/kindle-backup/files?user=1&device=Test+Kindle", nil, false)
	files, _ := lf["files"].([]any)
	if len(files) != 4 {
		t.Fatalf("the files are listed: %v", lf)
	}
	locked := 0
	for _, f := range files {
		if f.(map[string]any)["locked"] == true {
			locked++
		}
	}
	if locked != 2 {
		t.Fatalf("the two store books are marked locked: %v", lf)
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
			{"path": "/mnt/us/pillowd_2264_001-juno_18050001_sangria_bellatrix4-455680_crash_Jun_1.zip", "size": 9000, "mtime": 5},
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

// Tidy my Kindle's books: the copy on the server moves the same way, never overwrites, and an unknown file is ignored.
func TestKindleMovedFollowsTheDevice(t *testing.T) {
	dir := t.TempDir()
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.DataDir = dir })
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, ks := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	key := hex.EncodeToString(sum[:])
	root := filepath.Join(dir, "kindle-backup", "1", "Test Kindle")
	_ = os.MkdirAll(filepath.Join(root, "documents", "Dune.sdr"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "documents", "Dune.epub"), []byte("book"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "documents", "Dune.sdr", "metadata.epub.lua"), []byte("x"), 0o644)
	_ = os.MkdirAll(filepath.Join(root, "Books", "Frank Herbert"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "Books", "Frank Herbert", "Taken.epub"), []byte("other"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "documents", "Taken.epub"), []byte("mine"), 0o644)
	b, _ := json.Marshal(map[string]any{"device": "Test Kindle", "moves": []map[string]string{
		{"from": "/mnt/us/documents/Dune.epub", "to": "/mnt/us/Books/Frank Herbert/Dune.epub"},
		{"from": "/mnt/us/documents/Dune.sdr", "to": "/mnt/us/Books/Frank Herbert/Dune.sdr"},
		{"from": "/mnt/us/documents/Taken.epub", "to": "/mnt/us/Books/Frank Herbert/Taken.epub"},
		{"from": "/mnt/us/documents/Nothing.epub", "to": "/mnt/us/Books/x/Nothing.epub"},
		{"from": "/mnt/us/documents/../../etc/passwd", "to": "/mnt/us/Books/x/p"},
	}})
	req, _ := http.NewRequest("POST", srv.URL+"/kosync/plugin/kindle/moved", bytes.NewReader(b))
	req.Header.Set("x-auth-user", "admin")
	req.Header.Set("x-auth-key", key)
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("moved: %v %v", err, res)
	}
	var out map[string]int
	_ = json.NewDecoder(res.Body).Decode(&out)
	if out["moved"] != 2 {
		t.Fatalf("the book and its folder moved, the taken name and the missing file did not: %v", out)
	}
	if _, err := os.Stat(filepath.Join(root, "Books", "Frank Herbert", "Dune.sdr", "metadata.epub.lua")); err != nil {
		t.Fatalf("the sidecar folder came along: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "Books", "Frank Herbert", "Taken.epub")); string(got) != "other" {
		t.Fatal("nothing is overwritten")
	}
}

// A file sent in pieces: the pieces must follow each other, a wrong offset says where to carry on, the last piece
// completes the file and a file sent whole in one go is still accepted.
func TestKindleChunks(t *testing.T) {
	dir := t.TempDir()
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.DataDir = dir })
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, ks := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	key := hex.EncodeToString(sum[:])
	put := func(q string, body []byte) (int, map[string]any) {
		req, _ := http.NewRequest("PUT", srv.URL+"/kosync/plugin/kindle/chunk?"+q, bytes.NewReader(body))
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", key)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		return res.StatusCode, out
	}
	base := "device=Test+Kindle&mode=backup&mtime=50&path=/mnt/us/koreader/settings/big.lua&total=10"
	if c, out := put(base+"&offset=0", []byte("01234")); c != 200 || out["done"] != false || out["received"] != 5.0 {
		t.Fatalf("first piece: %d %v", c, out)
	}
	if c, out := put(base+"&offset=2", []byte("xx")); c != 409 || out["have"] != 5.0 {
		t.Fatalf("a wrong offset says where to carry on: %d %v", c, out)
	}
	if c, out := put(base+"&offset=5", []byte("56789")); c != 200 || out["done"] != true {
		t.Fatalf("last piece: %d %v", c, out)
	}
	got, err := os.ReadFile(filepath.Join(dir, "kindle-backup", "1", "Test Kindle", "koreader", "settings", "big.lua"))
	if err != nil || string(got) != "0123456789" {
		t.Fatalf("the file is whole: %q %v", got, err)
	}
	if fi, _ := os.Stat(filepath.Join(dir, "kindle-backup", "1", "Test Kindle", "koreader", "settings", "big.lua")); fi == nil || fi.ModTime().Unix() != 50 {
		t.Fatal("the change time is the Kindle's")
	}
	if c, _ := put("device=Test+Kindle&mode=backup&path=/mnt/us/koreader/cache/x.lua&total=1&offset=0", []byte("x")); c != 400 {
		t.Fatalf("a path the backup leaves out is refused: %d", c)
	}
}

// Kindle tasks: queued in the app, handed to the Kindle at its next check-in, reported back; a delete is refused for
// a book with no backup copy on the server, and only the fixed kinds exist.
func TestKindleTasks(t *testing.T) {
	dir := t.TempDir()
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.DataDir = dir })
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, ks := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	key := hex.EncodeToString(sum[:])
	plug := func(method, path string, body any) map[string]any {
		var rd *bytes.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		} else {
			rd = bytes.NewReader(nil)
		}
		req, _ := http.NewRequest(method, srv.URL+"/kosync/plugin/"+path, rd)
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
	// the Kindle reports two books; only one has a copy on the server
	plug("POST", "device", map[string]any{"device": "Test Kindle", "books": []map[string]any{
		{"filename": "Kept.epub", "title": "Kept", "format": "epub", "size": 5000},
		{"filename": "Loose.epub", "title": "Loose", "format": "epub", "size": 6000}}})
	root := filepath.Join(dir, "kindle-backup", "1", "Test Kindle", "Books")
	_ = os.MkdirAll(root, 0o755)
	_ = os.WriteFile(filepath.Join(root, "Kept.epub"), bytes.Repeat([]byte("k"), 5000), 0o644)

	if res, _ := admin.do("POST", "/api/admin/kindle-tasks", map[string]any{"kind": "format_disk"}, true); res.StatusCode != 400 {
		t.Fatalf("only the fixed kinds exist: %d", res.StatusCode)
	}
	if res, out := admin.do("POST", "/api/admin/kindle-tasks", map[string]any{"device": "Test Kindle", "kind": "delete_books", "filenames": []string{"Kept.epub", "Loose.epub"}}, true); res.StatusCode != 409 {
		t.Fatalf("a book with no backup copy is refused: %d %v", res.StatusCode, out)
	}
	if res, _ := admin.do("POST", "/api/admin/kindle-tasks", map[string]any{"device": "Test Kindle", "kind": "delete_books", "filenames": []string{"Kept.epub"}}, true); res.StatusCode != 200 {
		t.Fatalf("a backed-up book can be deleted: %d", res.StatusCode)
	}
	admin.do("POST", "/api/admin/kindle-tasks", map[string]any{"kind": "crashlogs"}, true)
	_, list := admin.do("GET", "/api/admin/kindle-tasks?device=Test+Kindle", nil, false)
	files, _ := list["files"].([]any)
	if len(files) != 2 || files[0].(map[string]any)["backed_up"] != true || files[1].(map[string]any)["backed_up"] != false {
		t.Fatalf("the app shows which books are backed up: %v", list["files"])
	}
	got := plug("GET", "tasks?device=Test+Kindle", nil)
	tasks, _ := got["tasks"].([]any)
	if len(tasks) != 2 || tasks[0].(map[string]any)["kind"] != "delete_books" {
		t.Fatalf("the Kindle is handed both tasks: %v", got)
	}
	id := tasks[0].(map[string]any)["id"].(float64)
	plug("POST", "tasks/result", map[string]any{"id": id, "status": "done", "note": "deleted 1"})
	if again := plug("GET", "tasks?device=Test+Kindle", nil); len(again["tasks"].([]any)) != 1 {
		t.Fatalf("a done task is not handed out again: %v", again)
	}
	// taking one back
	left := plug("GET", "tasks?device=Test+Kindle", nil)["tasks"].([]any)[0].(map[string]any)["id"].(float64)
	if res, _ := admin.do("DELETE", "/api/admin/kindle-tasks/"+itoa(int64(left)), nil, true); res.StatusCode != 200 {
		t.Fatal("a waiting task can be taken back")
	}
	if again := plug("GET", "tasks?device=Test+Kindle", nil); len(again["tasks"].([]any)) != 0 {
		t.Fatalf("a task taken back is not handed out: %v", again)
	}
}
