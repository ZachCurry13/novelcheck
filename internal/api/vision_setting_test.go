package api_test

import "testing"

func TestPhotoModelSettingIsAccepted(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	if res, out := admin.do("PUT", "/api/admin/settings", map[string]string{"vision_model": "gemma3:4b"}, true); res.StatusCode != 200 {
		t.Fatalf("save: %d %v", res.StatusCode, out)
	}
	if st.Setting("vision_model") != "gemma3:4b" {
		t.Fatalf("kept: %q", st.Setting("vision_model"))
	}
}
