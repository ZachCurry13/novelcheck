package titles

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in     string
		title  string
		series string
		index  float64
	}{
		// Track and series numbers in front.
		{"01 - The Hobbit", "The Hobbit", "", 1},
		{"02 The Two Towers", "The Two Towers", "", 2},
		{"003_Return of the King", "Return of the King", "", 3},
		{"1 - Dune", "Dune", "", 1},
		{"4. Dune Messiah", "Dune Messiah", "", 4},
		{"2.5 - A Novella", "A Novella", "", 2.5},
		{"#3: Children of Dune", "Children of Dune", "", 3},
		{"Book 2: Catching Fire", "Catching Fire", "", 2},
		{"Book Two - Catching Fire", "Catching Fire", "", 2},
		{"Vol. 3 Mockingjay", "Mockingjay", "", 3},
		// Series in brackets or after the title.
		{"Guards! Guards! (Discworld, #8)", "Guards! Guards!", "Discworld", 8},
		{"The Fellowship of the Ring (The Lord of the Rings Book 1)", "The Fellowship of the Ring", "The Lord of the Rings", 1},
		{"Mistborn [Mistborn #1]", "Mistborn", "Mistborn", 1},
		{"Dune Messiah (Book 2 of 6)", "Dune Messiah", "", 2},
		{"Catching Fire, Book Two", "Catching Fire", "", 2},
		{"Catching Fire - Vol 2", "Catching Fire", "", 2},
		{"01 - Dune (Dune Chronicles, #1)", "Dune", "Dune Chronicles", 1},
		// Numbers that belong to the title stay.
		{"1984", "1984", "", 0},
		{"13 Reasons Why", "13 Reasons Why", "", 0},
		{"4-Hour Workweek", "4-Hour Workweek", "", 0},
		{"2001: A Space Odyssey", "2001: A Space Odyssey", "", 0},
		{"11/22/63", "11/22/63", "", 0},
		{"20,000 Leagues Under the Sea", "20,000 Leagues Under the Sea", "", 0},
		{"The Book Thief", "The Book Thief", "", 0},
		{"Book Club Picks (Book Club Edition)", "Book Club Picks (Book Club Edition)", "", 0},
		{"Book 2", "Book 2", "", 0},
		{"  Plain   Title  ", "Plain Title", "", 0},
	}
	for _, c := range cases {
		got := Parse(c.in)
		if got.Title != c.title || got.Series != c.series || got.Index != c.index {
			t.Errorf("Parse(%q) = %+v, want %q / %q / %v", c.in, got, c.title, c.series, c.index)
		}
	}
}

func TestFormatIndex(t *testing.T) {
	if FormatIndex(2) != "2" || FormatIndex(2.5) != "2.5" {
		t.Fatal(FormatIndex(2), FormatIndex(2.5))
	}
}
