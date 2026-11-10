package api

import "testing"

func TestSameCalibreTitle(t *testing.T) {
	for _, p := range [][2]string{
		{"A Kiss of Shadows: A Merry Gentry Novel", "A Kiss of Shadows"},
		{"Haunted: a novel of stories", "Haunted"},
		{"Cinder", "cinder"},
		{"Rant", "Rant: An Oral Biography of Buster Casey"},
		{"It (Movie Tie-in)", "It"},
	} {
		if !sameCalibreTitle(p[0], p[1]) {
			t.Errorf("%q is %q", p[0], p[1])
		}
	}
	for _, p := range [][2]string{{"Cinder", "Scarlet"}, {"It", "It Ends with Us"}} {
		if sameCalibreTitle(p[0], p[1]) {
			t.Errorf("%q is not %q", p[0], p[1])
		}
	}
}
