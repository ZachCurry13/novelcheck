package api_test

import "testing"

// Classic blue can be picked as a theme, on its own or as dark or light; an unknown theme is refused.
func TestClassicBlueTheme(t *testing.T) {
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	for _, theme := range []string{"blue", "blue-dark", "blue-light", "", "dark", "light"} {
		if res, _ := admin.do("PUT", "/api/me/appearance", map[string]string{"theme": theme}, true); res.StatusCode != 200 {
			t.Fatalf("%q: %d", theme, res.StatusCode)
		}
	}
	if res, _ := admin.do("PUT", "/api/me/appearance", map[string]string{"theme": "purple"}, true); res.StatusCode != 400 {
		t.Fatalf("unknown theme: %d", res.StatusCode)
	}
}
