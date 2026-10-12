package titles

import "testing"

func TestFromFilename(t *testing.T) {
	cases := []struct {
		in    string
		first Guess
		last  Guess
	}{
		{"The Catcher in the Rye - J.D. Salinger_ef758c89-e01f-4561-a91a-c8b93b748491.epub",
			Guess{"The Catcher in the Rye", "J.D. Salinger"}, Guess{Title: "The Catcher in the Rye - J.D. Salinger"}},
		{"Lemony_Snicket_-_The_Hostile_Hospital.epub", Guess{"Lemony Snicket", "The Hostile Hospital"}, Guess{Title: "Lemony Snicket - The Hostile Hospital"}},
		{"Prayers-of-the-Rosary.epub", Guess{Title: "Prayers-of-the-Rosary"}, Guess{Title: "Prayers-of-the-Rosary"}},
		{"Dune (z-lib.org).epub", Guess{Title: "Dune"}, Guess{Title: "Dune"}},
	}
	for _, c := range cases {
		g := FromFilename(c.in)
		if len(g) == 0 || g[0] != c.first || g[len(g)-1] != c.last {
			t.Errorf("%q = %+v, want first %+v, last %+v", c.in, g, c.first, c.last)
		}
	}
	if g := FromFilename("  .epub "); g != nil {
		t.Errorf("empty name: %+v", g)
	}
	// The author-last-name check in MatchBook turns the right reading into a match;
	// both orders are offered.
	g := FromFilename("Frank Herbert - Dune.epub")
	if len(g) < 3 || g[0].Title != "Frank Herbert" || g[1] != (Guess{"Dune", "Frank Herbert"}) {
		t.Errorf("both readings should be offered: %+v", g)
	}
}
