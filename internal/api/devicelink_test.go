package api_test

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The plugin's report and the app's of one Kindle are linked: by name when the app was named at download, or by
// the person; a person can only link their own e-readers.
func TestPluginAndAppAreOneKindle(t *testing.T) {
	srv, st := setup(t)
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]any{"username": "wife", "password": "wifepass12", "role": "editor"}, true)
	wife := login(t, srv, "wife", "wifepass12")
	_, ks := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	report := func(device, version string) {
		raw, _ := json.Marshal(map[string]any{"device": device, "plugin_version": version, "books": []any{}})
		req, _ := http.NewRequest("POST", srv.URL+"/kosync/plugin/device", bytes.NewReader(raw))
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", hex.EncodeToString(sum[:]))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
	}
	report("Zach Paperwhite", "1.64.0")
	report("Zach Paperwhite (Amazon reader)", "app 1.3.2")
	_, rs := admin.do("GET", "/api/me/reading-sync", nil, false)
	links, _ := rs["links"].(map[string]any)
	if links["Zach Paperwhite"] != "Zach Paperwhite" || links["Zach Paperwhite (Amazon reader)"] != "Zach Paperwhite" {
		t.Fatalf("named alike: linked by themselves: %v", rs["links"])
	}
	report("Kindle (Amazon reader)", "app 1.3.2")
	if res, _ := admin.do("PUT", "/api/me/device-link", map[string]any{"device": "Kindle (Amazon reader)", "kindle": "Zach Paperwhite"}, true); res.StatusCode != 200 {
		t.Fatalf("link by hand: %d", res.StatusCode)
	}
	if res, _ := admin.do("PUT", "/api/me/device-link", map[string]any{"device": "Kindle (Amazon reader)", "kindle": ""}, true); res.StatusCode != 200 {
		t.Fatal("unlink")
	}
	if res, _ := wife.do("PUT", "/api/me/device-link", map[string]any{"device": "Zach Paperwhite", "kindle": "Mine now"}, true); res.StatusCode != 400 {
		t.Fatalf("someone else's e-reader can't be linked: %d", res.StatusCode)
	}
}
