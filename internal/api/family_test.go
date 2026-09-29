package api_test

import (
	"strings"
	"testing"
)

// A family device: kids tap in, parents need their PIN or password, wrong
// PINs lock the profile, and only parents can mark or unmark devices.
func TestFamilyDevice(t *testing.T) {
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	_, kid := admin.do("POST", "/api/admin/users", map[string]any{"username": "tween", "password": "kidpass12", "age_level": 2}, true)
	admin.do("POST", "/api/admin/users", map[string]string{"username": "parent", "password": "editorpass1", "role": "editor"}, true)
	kidID := kid["id"].(float64)
	ids := map[string]float64{}

	// Not a family device yet: no profiles, no switching.
	if _, f := admin.do("GET", "/api/family", nil, false); f["family"] != false {
		t.Fatalf("not a family device: %v", f)
	}
	if res, _ := admin.do("POST", "/api/family/switch", map[string]any{"user_id": kidID}, true); res.StatusCode != 403 {
		t.Fatalf("switch on an ordinary device: %d", res.StatusCode)
	}
	// Kids can't mark a device.
	kidClient := login(t, srv, "tween", "kidpass12")
	if res, _ := kidClient.do("POST", "/api/me/family-device", map[string]string{"name": "Tablet"}, true); res.StatusCode != 403 {
		t.Fatalf("a kid marking a device: %d", res.StatusCode)
	}

	if res, _ := admin.do("POST", "/api/me/family-device", map[string]string{"name": "Kitchen tablet"}, true); res.StatusCode != 200 {
		t.Fatalf("mark: %d", res.StatusCode)
	}
	_, f := admin.do("GET", "/api/family", nil, false)
	if f["family"] != true || f["name"] != "Kitchen tablet" {
		t.Fatalf("family: %v", f)
	}
	need := map[string]string{}
	for _, p := range f["profiles"].([]any) {
		m := p.(map[string]any)
		need[m["username"].(string)] = m["needs"].(string)
		ids[m["username"].(string)] = m["id"].(float64)
	}
	if need["admin"] != "password" || need["parent"] != "password" || need["tween"] != "none" {
		t.Fatalf("what each profile needs: %v", need)
	}

	// A kid taps in; the device stays a family device.
	res, me := admin.do("POST", "/api/family/switch", map[string]any{"user_id": kidID}, true)
	if res.StatusCode != 200 || me["username"] != "tween" || me["family_device"] != true {
		t.Fatalf("kid tap: %d %v", res.StatusCode, me)
	}
	// Back to a parent: the password, since there's no PIN yet.
	if res, _ := admin.do("POST", "/api/family/switch", map[string]any{"user_id": ids["admin"], "password": "nope"}, true); res.StatusCode != 401 {
		t.Fatalf("wrong password: %d", res.StatusCode)
	}
	if res, me = admin.do("POST", "/api/family/switch", map[string]any{"user_id": ids["admin"], "password": "adminpass1"}, true); res.StatusCode != 200 || me["username"] != "admin" {
		t.Fatalf("parent with password: %d %v", res.StatusCode, me)
	}

	// A PIN: set with the password, then it's what the profile asks for.
	if res, _ := admin.do("PUT", "/api/me/pin", map[string]string{"pin": "1234", "password": "wrong"}, true); res.StatusCode != 403 {
		t.Fatalf("PIN without the password: %d", res.StatusCode)
	}
	if res, _ := admin.do("PUT", "/api/me/pin", map[string]string{"pin": "12a4", "password": "adminpass1"}, true); res.StatusCode != 400 {
		t.Fatalf("a PIN is 4 digits: %d", res.StatusCode)
	}
	if res, out := admin.do("PUT", "/api/me/pin", map[string]string{"pin": "1234", "password": "adminpass1"}, true); res.StatusCode != 200 || out["has_pin"] != true {
		t.Fatalf("set PIN: %d %v", res.StatusCode, out)
	}
	admin.do("POST", "/api/family/switch", map[string]any{"user_id": kidID}, true)
	for i := 0; i < 5; i++ {
		if res, _ := admin.do("POST", "/api/family/switch", map[string]any{"user_id": ids["admin"], "pin": "0000"}, true); res.StatusCode != 401 {
			t.Fatalf("wrong PIN %d: %d", i, res.StatusCode)
		}
	}
	res, out := admin.do("POST", "/api/family/switch", map[string]any{"user_id": ids["admin"], "pin": "1234"}, true)
	if res.StatusCode != 429 || !strings.Contains(out["error"].(string), "minutes") {
		t.Fatalf("locked after 5 wrong PINs, even with the right one: %d %v", res.StatusCode, out)
	}
	// Still signed in as the kid, who can't touch PINs or devices.
	if _, me := admin.do("GET", "/api/me", nil, false); me["username"] != "tween" {
		t.Fatalf("who's signed in: %v", me)
	}
	if res, _ := admin.do("PUT", "/api/admin/users/"+itoa(int64(ids["admin"]))+"/pin", map[string]string{"pin": ""}, true); res.StatusCode != 403 {
		t.Fatalf("a kid clearing a parent's PIN: %d", res.StatusCode)
	}

	// Another parent (signed in elsewhere) removes the device: no more profiles.
	editor := login(t, srv, "parent", "editorpass1")
	_, list := editor.doList("GET", "/api/admin/family-devices")
	if len(list) != 1 {
		t.Fatalf("devices: %v", list)
	}
	if res, _ := editor.do("PUT", "/api/admin/users/"+itoa(int64(ids["admin"]))+"/pin", map[string]string{"pin": ""}, true); res.StatusCode != 403 {
		t.Fatalf("an editor clearing the admin's PIN: %d", res.StatusCode)
	}
	if res, _ := editor.do("PUT", "/api/admin/users/"+itoa(int64(kidID))+"/pin", map[string]string{"pin": "4321"}, true); res.StatusCode != 200 {
		t.Fatalf("an editor setting a kid's PIN: %d", res.StatusCode)
	}
	editor.do("DELETE", "/api/admin/family-devices/"+itoa(int64(list[0]["id"].(float64))), nil, true)
	if _, f := admin.do("GET", "/api/family", nil, false); f["family"] != false {
		t.Fatalf("removed device: %v", f)
	}
}
