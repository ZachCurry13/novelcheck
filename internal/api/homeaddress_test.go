package api

import "testing"

func TestHomeAddress(t *testing.T) {
	for raw, want := range map[string]string{
		"http://192.168.1.20:8080/": "http://192.168.1.20:8080",
		" https://nas.local ":       "https://nas.local",
		"":                          "",
	} {
		if got, ok := homeAddress(raw); !ok || got != want {
			t.Errorf("%q: got %q %v", raw, got, ok)
		}
	}
	for _, bad := range []string{"192.168.1.20", "ftp://x", "http://a/b", "http://u:p@x", "javascript:alert(1)", "http://x?y=1"} {
		if _, ok := homeAddress(bad); ok {
			t.Errorf("%q should be refused", bad)
		}
	}
}
