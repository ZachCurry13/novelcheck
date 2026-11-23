package api

import "testing"

func TestSameCalibreTitle(t *testing.T) {
	for _, p := range [][2]string{
		{"A Kiss of Shadows: A Merry Gentry Novel", "A Kiss of Shadows"},
		{"Haunted: a novel of stories", "Haunted"},
		{"Cinder", "cinder"},
		{"Rant", "Rant: An Oral Biography of Buster Casey"},
		{"It (Movie Tie-in)", "It"},
		{"An American Sickness - Elisabeth Rosenthal", "An American Sickness: How Healthcare Became Big Business and How You Can Take It Back"},
		{"Bad Beginning - Snicket, Lemony", "The Bad Beginning"},
		{"Shadow of Kyoshi - F. C. Yee", "The Shadow of Kyoshi"},
		{"Vile Village - Snicket, Lemony", "The Vile Village"},
	} {
		if !sameCalibreTitle(p[0], p[1]) {
			t.Errorf("%q is %q", p[0], p[1])
		}
	}
	for _, p := range [][2]string{{"Cinder", "Scarlet"}, {"It", "It Ends with Us"}, {"Reptile Room - Snicket, Lemony", "The Bad Beginning"}, {"Dune - Frank Herbert", "Dune Messiah"}} {
		if sameCalibreTitle(p[0], p[1]) {
			t.Errorf("%q is not %q", p[0], p[1])
		}
	}
}
