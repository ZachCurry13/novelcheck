package enrich

import "testing"

func TestSure(t *testing.T) {
	inkheart := Choice{Title: "Inkheart", Author: "Cornelia Funke"}
	twain := Choice{Title: "The Adventures of Tom Sawyer", Author: "Mark Twain"}
	hobbit := Choice{Title: "The Hobbit", Author: "J.R.R. Tolkien"}
	cases := []struct {
		query   string
		choices []Choice
		want    bool
	}{
		{"ink world by funke", []Choice{twain, inkheart}, false}, // the first result doesn't match what was typed
		{"inkheart funke", []Choice{inkheart, twain}, true},      // every word, one match
		{"the hobbit tolkien", []Choice{hobbit}, true},           // "the" needn't match
		{"harry potter", []Choice{{Title: "Harry Potter and the Chamber of Secrets", Author: "J.K. Rowling"},
			{Title: "Harry Potter and the Goblet of Fire", Author: "J.K. Rowling"}}, false}, // several fit
		{"Harry Potter goblet", []Choice{{Title: "Harry Potter and the Goblet of Fire", Author: "J.K. Rowling"},
			{Title: "Harry Potter and the Chamber of Secrets", Author: "J.K. Rowling"}}, true},
		{"inkworld", []Choice{{Title: "Ink World", Author: "Someone"}}, true}, // spaces don't matter
		{"anything", nil, false},
	}
	for _, c := range cases {
		if got := Sure(c.query, c.choices); got != c.want {
			t.Errorf("%q: %v, want %v", c.query, got, c.want)
		}
	}
}
