package api_test

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The plugin's "Send log to NovelCheck": stored without the sync code, listed
// and read by admins only, and only the last few per device are kept.
func TestDeviceLogs(t *testing.T) {
	srv, st := setup(t)
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, c := admin.do("GET", "/api/me/kosync", nil, false)
	code := c["code"].(string)
	sum := md5.Sum([]byte(code))
	key := hex.EncodeToString(sum[:])
	send := func(k, text string) int {
		body := `{"device":"Paperwhite","version":"1.48.1","text":` + jsonString(text) + `}`
		req, _ := http.NewRequest("POST", srv.URL+"/kosync/plugin/debug", bytes.NewBufferString(body))
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", k)
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if send("wrong", "x") != 401 {
		t.Fatal("a wrong key must not be accepted")
	}
	if send(key, "line one with code "+code+" in it") != 200 {
		t.Fatal("send")
	}
	for i := 0; i < 10; i++ {
		send(key, "more")
	}
	_, list := admin.do("GET", "/api/admin/device-logs", nil, false)
	logs := list["logs"].([]any)
	if len(logs) != 8 || logs[0].(map[string]any)["device"] != "Paperwhite" {
		t.Fatalf("the last 8 per device are kept: %d %v", len(logs), list)
	}
	oldest := logs[len(logs)-1].(map[string]any)["id"].(float64)
	if first, _ := st.DeviceLogText(int64(oldest)); strings.Contains(first, code) {
		t.Fatal("the sync code must be blanked out")
	}
	if res, _ := admin.do("DELETE", "/api/admin/device-logs/"+itoa(int64(oldest)), nil, true); res.StatusCode != 200 {
		t.Fatal("delete")
	}
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	if res, _ := kid.do("GET", "/api/admin/device-logs", nil, false); res.StatusCode != 403 {
		t.Fatalf("only admins read device logs: %d", res.StatusCode)
	}
}

func jsonString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s) + `"`
}
