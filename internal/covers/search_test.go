package covers

import "testing"

// A title copied from a list ("Title - Author - Year", the list's name as the author) is split so the book can be found.
func TestSplitListTitle(t *testing.T) {
	for _, c := range []struct{ title, author, wantT, wantA string }{
		{"Neuromancer - William Gibson - 1984", "Top 100 Sci-Fi Books", "Neuromancer", "William Gibson"},
		{"The Mote in God's Eye - Niven & Pournelle - 1975", "Top 100 Sci-Fi Books", "The Mote in God's Eye", "Niven & Pournelle"},
		{"Rendezvous With Rama – Arthur C Clarke – 1973", "x", "Rendezvous With Rama", "Arthur C Clarke"},
		{"Percy Jackson 2 - The Sea of Monsters", "Rick Riordan", "Percy Jackson 2 - The Sea of Monsters", "Rick Riordan"},
		{"Prologue - Roont", "Stephen King", "Prologue - Roont", "Stephen King"},
		{"Fahrenheit 451", "Ray Bradbury", "Fahrenheit 451", "Ray Bradbury"},
	} {
		if gt, ga := SplitListTitle(c.title, c.author); gt != c.wantT || ga != c.wantA {
			t.Errorf("%q: got %q by %q, want %q by %q", c.title, gt, ga, c.wantT, c.wantA)
		}
	}
}
