package titles

import "testing"

func TestTieIns(t *testing.T) {
	for in, want := range map[string]string{
		"Shadow and Bone (Netflix Tie-In)":           "Shadow and Bone",
		"The Queen's Gambit (Television Tie-in)":     "The Queen's Gambit",
		"Wonder (Movie Tie-In Edition)":              "Wonder",
		"Big Little Lies (TV Tie-in): A Novel":       "Big Little Lies",
		"Where the Crawdads Sing: Movie Tie-In":      "Where the Crawdads Sing",
		"Dune (Now a Major Motion Picture)":          "Dune",
		"Outlander [TV Series Tie-In]":               "Outlander",
		"The Hunger Games (Tie-in Edition)":          "The Hunger Games",
		"Bridgerton (Now a Netflix Original Series)": "Bridgerton",
		"Tied In Knots":                              "Tied In Knots",
		"The Tie-In":                                 "The Tie-In",
		"Fourth Wing (The Empyrean, #1) (TV Tie-In)": "Fourth Wing",
	} {
		if got := Clean(in); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
}
