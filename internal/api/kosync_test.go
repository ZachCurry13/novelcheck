package api_test

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/kosync"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// KOReader signs in with the NovelCheck name and sync code, sends where it
// is in a book, and the book moves along in Up Next.
func TestKOReaderSync(t *testing.T) {
	var calDir string
	srv, st := setupWith(t, func(s *api.Server) { calDir = s.Cfg.CalibreDir })
	admin := login(t, srv, "admin", "adminpass1")
	file := filepath.Join(calDir, "Author", "Book (1)", "book.epub")
	_ = os.MkdirAll(filepath.Dir(file), 0o755)
	_ = os.WriteFile(file, bytes.Repeat([]byte("a chapter of a book. "), 3000), 0o644)
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	id, _ := st.UpsertBook("Synced Book", "Author", "", "")
	_ = st.AddCopy(cal, id, file, "epub", "1")
	u, _ := st.UserByName("admin")
	_ = st.Enqueue(u.ID, id)

	_, me := admin.do("GET", "/api/me/kosync", nil, false)
	code, _ := me["code"].(string)
	if len(code) != 12 || me["path"] != "/kosync" {
		t.Fatalf("code: %v", me)
	}
	sum := md5.Sum([]byte(code))
	key := hex.EncodeToString(sum[:])
	call := func(method, path, key string, body any) (*http.Response, map[string]any) {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req, _ := http.NewRequest(method, srv.URL+path, &buf)
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", key)
		req.Header.Set("accept", "application/vnd.koreader.v1+json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		out := map[string]any{}
		_ = json.NewDecoder(res.Body).Decode(&out)
		return res, out
	}
	if res, _ := call("GET", "/kosync/users/auth", key, nil); res.StatusCode != 200 {
		t.Fatalf("auth: %d", res.StatusCode)
	}
	if res, _ := call("GET", "/kosync/users/auth", "0123", nil); res.StatusCode != 401 {
		t.Fatalf("wrong key: %d", res.StatusCode)
	}
	if res, _ := call("POST", "/kosync/users/create", "", map[string]string{"username": "admin", "password": key}); res.StatusCode != 201 {
		t.Fatalf("register with the code: %d", res.StatusCode)
	}
	doc, _ := kosync.PartialMD5(file)
	status := func() string {
		var s string
		_ = st.DB.Get(&s, `SELECT status FROM queue_items WHERE book_id = ?`, id)
		return s
	}
	res, out := call("PUT", "/kosync/syncs/progress", key, map[string]any{"document": doc, "progress": "/body/DocFragment[3]", "percentage": 0.4, "device": "Kobo", "device_id": "k1"})
	if res.StatusCode != 200 || out["document"] != doc || status() != "reading" {
		t.Fatalf("progress: %d %v, status %q", res.StatusCode, out, status())
	}
	if _, out = call("GET", "/kosync/syncs/progress/"+doc, key, nil); out["percentage"] != 0.4 || out["device"] != "Kobo" {
		t.Fatalf("get: %v", out)
	}
	call("PUT", "/kosync/syncs/progress", key, map[string]any{"document": doc, "progress": "end", "percentage": 0.99, "device": "Kobo", "device_id": "k1"})
	if status() != "finished" {
		t.Fatalf("at the end: %q", status())
	}
	call("PUT", "/kosync/syncs/progress", key, map[string]any{"document": doc, "progress": "start", "percentage": 0.01, "device": "Kobo", "device_id": "k1"})
	if status() != "finished" {
		t.Fatal("reading it again must not undo Finished")
	}
	_ = st.SetSetting(store.KeyModuleKOReader, "false")
	if res, _ := call("GET", "/kosync/users/auth", key, nil); res.StatusCode != 401 {
		t.Fatalf("switched off: %d", res.StatusCode)
	}
}

// A Goodreads or StoryGraph export's "read" shelf counts as finished.
func TestImportReadShelf(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	res, _ := admin.do("POST", "/api/import/drive", map[string]any{"catalog_name": "Goodreads", "books": []map[string]any{
		{"title": "Read Book", "author": "A. Writer", "format": "list", "path": "list:Read Book | A. Writer", "read": true},
		{"title": "Wanted Book", "author": "A. Writer", "format": "list", "path": "list:Wanted Book | A. Writer"},
	}}, true)
	if res.StatusCode != 200 {
		t.Fatalf("import: %d", res.StatusCode)
	}
	var finished, other int
	_ = st.DB.Get(&finished, `SELECT COUNT(*) FROM queue_items q JOIN books b ON b.id = q.book_id WHERE b.title = 'Read Book' AND q.status = 'finished'`)
	_ = st.DB.Get(&other, `SELECT COUNT(*) FROM queue_items q JOIN books b ON b.id = q.book_id WHERE b.title = 'Wanted Book'`)
	if finished != 1 || other != 0 {
		t.Fatalf("finished %d, other %d", finished, other)
	}
}
