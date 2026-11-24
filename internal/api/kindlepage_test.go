package api_test

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The Kindle page lists one Kindle's books from its plugin report and its app report together, a shared book once.
func TestKindlePageMergesPluginAndApp(t *testing.T) {
	srv, st := setup(t)
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, ks := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	report := func(device, books string) {
		req, _ := http.NewRequest("POST", srv.URL+"/kosync/plugin/device", strings.NewReader(`{"device":"`+device+`","plugin_version":"1.0.0","books":`+books+`}`))
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", hex.EncodeToString(sum[:]))
		if res, err := http.DefaultClient.Do(req); err != nil || res.StatusCode != 200 {
			t.Fatalf("report: %v", err)
		}
	}
	report("Paperwhite", `[{"filename":"a.epub","title":"Alpha","author":"Ann","format":"epub","size":1},{"filename":"b.epub","title":"Beta","author":"Bob","format":"epub","size":1}]`)
	report("Paperwhite (Amazon reader)", `[{"filename":"a.azw3","title":"Alpha","author":"Ann","format":"azw3","size":1}]`)
	admin.do("PUT", "/api/me/device-link", map[string]string{"device": "Paperwhite (Amazon reader)", "kindle": "Paperwhite"}, false)
	_, r := admin.do("GET", "/api/me/kindle-library?kindle=Paperwhite", nil, false)
	if books, _ := r["books"].([]any); len(books) != 2 {
		t.Fatalf("want Alpha and Beta once each, got %v", r["books"])
	}
}
