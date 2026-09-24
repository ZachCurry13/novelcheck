package api_test

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestPushSubscriptions(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")

	res, out := admin.do("GET", "/api/push", nil, false)
	if res.StatusCode != 200 || len(out["public_key"].(string)) != 87 { // 65 bytes, base64url
		t.Fatalf("push status: %d %v", res.StatusCode, out)
	}

	ua, _ := ecdh.P256().GenerateKey(rand.Reader)
	keys := map[string]string{"p256dh": base64.RawURLEncoding.EncodeToString(ua.PublicKey().Bytes()), "auth": "BTBZMqHH6r4Tts7J_aSIgg"}
	sub := func(endpoint string) int {
		res, _ := admin.do("POST", "/api/push/subscribe", map[string]any{"endpoint": endpoint, "keys": keys, "scope": "problems", "device": "Test phone"}, true)
		return res.StatusCode
	}
	// Only real push services: never an address inside the home network.
	for _, bad := range []string{"http://fcm.googleapis.com/fcm/send/x", "https://192.168.1.1/admin", "https://evil.example/fcm.googleapis.com",
		"https://fcm.googleapis.com.evil.example/x", "https://fcm.googleapis.com:8443/x", "https://user@fcm.googleapis.com/x"} {
		if code := sub(bad); code != 400 {
			t.Fatalf("%s must be refused: %d", bad, code)
		}
	}
	good := "https://fcm.googleapis.com/fcm/send/abc123"
	if code := sub(good); code != 200 {
		t.Fatalf("subscribe: %d", code)
	}
	keys["auth"] = "short"
	if code := sub("https://web.push.apple.com/xyz"); code != 400 {
		t.Fatalf("bad auth secret must be refused: %d", code)
	}
	if _, out = admin.do("GET", "/api/push", nil, false); len(out["devices"].([]any)) != 1 {
		t.Fatalf("devices: %v", out["devices"])
	}
	if d := out["devices"].([]any)[0].(map[string]any); d["scope"] != "problems" || d["device"] != "Test phone" || d["endpoint"] != nil {
		t.Fatalf("device listing (no endpoint or keys): %v", d)
	}

	// The server's private key never leaves it.
	secret := st.Setting(store.KeyPushPrivateKey)
	if secret == "" {
		t.Fatal("key should be saved")
	}
	for _, path := range []string{"/api/admin/settings", "/api/admin/diagnostics"} {
		r, err := admin.http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if r.StatusCode != http.StatusOK || strings.Contains(string(body), secret[:40]) {
			t.Fatalf("%s leaked the push key (status %d)", path, r.StatusCode)
		}
	}
	if res, _ := admin.do("PUT", "/api/admin/settings", map[string]string{store.KeyPushPrivateKey: "x"}, true); res.StatusCode != 400 {
		t.Fatalf("the push key isn't editable: %d", res.StatusCode)
	}

	if res, _ := admin.do("POST", "/api/push/unsubscribe", map[string]string{"endpoint": good}, true); res.StatusCode != 200 {
		t.Fatalf("unsubscribe: %d", res.StatusCode)
	}
	if subs, _ := st.UserPushSubs(1); len(subs) != 0 {
		t.Fatalf("still subscribed: %+v", subs)
	}
	if res, _ := admin.do("POST", "/api/push/test", nil, true); res.StatusCode != 400 {
		t.Fatalf("test with no devices: %d", res.StatusCode)
	}
}
