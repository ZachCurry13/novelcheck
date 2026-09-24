package version

import "testing"

func TestNewer(t *testing.T) {
	cases := []struct {
		release, current string
		want             bool
	}{
		{"v1.1.0", "1.0.0", true},
		{"v1.0.0", "1.0.0", false},
		{"v1.0.0", "1.0.0-3-gabc1234", false}, // main build after 1.0.0
		{"v1.0.1", "1.0.0-3-gabc1234", true},
		{"v1.10.0", "1.9.9", true},
		{"v1.1.0", "dev", false},
		{"nightly", "1.0.0", false},
		{"v2.0.0", "v1.99.99", true},
	}
	for _, c := range cases {
		if got := Newer(c.release, c.current); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.release, c.current, got, c.want)
		}
	}
}
