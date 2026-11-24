package api_test

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Restore lists this e-reader's own backup and sends its files; nothing outside it is reachable.
func TestRestoreFromBackup(t *testing.T) {
	data := t.TempDir()
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.DataDir = data })
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, ks := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	u, _ := st.UserByName("admin")
	dev := filepath.Join(data, "kindle-backup", itoa(u.ID), "Paperwhite")
	_ = os.MkdirAll(filepath.Join(dev, "koreader", "settings"), 0o755)
	_ = os.WriteFile(filepath.Join(dev, "koreader", "settings", "progress.lua"), []byte("return {}"), 0o644)
	_ = os.WriteFile(filepath.Join(dev, "book.epub.part"), []byte("half"), 0o644)
	_ = os.WriteFile(filepath.Join(data, "secret.txt"), []byte("no"), 0o644)
	get := func(path string) (int, string) {
		req, _ := http.NewRequest("GET", srv.URL+path, nil)
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", hex.EncodeToString(sum[:]))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	if code, body := get("/kosync/plugin/restore?device=Paperwhite"); code != 200 || !strings.Contains(body, `"koreader/settings/progress.lua"`) || strings.Contains(body, ".part") {
		t.Fatalf("list: %d %s", code, body)
	}
	if code, body := get("/kosync/plugin/restore/file?device=Paperwhite&path=koreader/settings/progress.lua"); code != 200 || body != "return {}" {
		t.Fatalf("file: %d %q", code, body)
	}
	for _, bad := range []string{"../../secret.txt", "..%2F..%2Fsecret.txt", "/mnt/us/../../../secret.txt"} {
		if code, _ := get("/kosync/plugin/restore/file?device=Paperwhite&path=" + bad); code != 404 {
			t.Fatalf("%s: %d", bad, code)
		}
	}
	if code, _ := get("/kosync/plugin/restore/file?device=..&path=secret.txt"); code != 404 {
		t.Fatalf("another folder: %d", code)
	}
}
