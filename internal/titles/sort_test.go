package titles

import "testing"

func TestSortKeys(t *testing.T) {
	for in, want := range map[string]string{
		"The Hobbit": "hobbit", "A Wrinkle in Time": "wrinkle in time", "An Abundance of Katherines": "abundance of katherines",
		"Anne of Green Gables": "anne of green gables", "'Salem's Lot": "salem's lot", "The": "the", "Theodore Boone": "theodore boone",
	} {
		if got := SortTitle(in); got != want {
			t.Errorf("SortTitle(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"Terry Pratchett": "pratchett, terry", "Terry Pratchett & Neil Gaiman": "pratchett, terry",
		"J.R.R. Tolkien": "tolkien, j.r.r.", "Ursula K. Le Guin": "le guin, ursula k.", "Pratchett, Terry": "pratchett, terry",
		"Martin Luther King Jr.": "king, martin luther jr.", "Martin Luther King, Jr.": "king, martin luther jr.",
		"Homer": "homer", "": "", "Ludwig van Beethoven": "van beethoven, ludwig",
	} {
		if got := SortAuthor(in); got != want {
			t.Errorf("SortAuthor(%q) = %q, want %q", in, got, want)
		}
	}
}
