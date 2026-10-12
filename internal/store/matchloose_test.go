package store_test

import "testing"

func TestMatchBookLoose(t *testing.T) {
	st := newStore(t)
	add := func(title, author string) int64 {
		id, err := st.UpsertBook(title, author, "", "")
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	hospital := add("The Hostile Hospital", "Lemony Snicket")
	reptile := add("The Reptile Room", "Lemony Snicket")
	add("The Reptile Room", "Someone Else")
	six := add("Six's Legacy", "Pittacus Lore")
	lost := add("The Lost Files: Hunted", "Pittacus Lore")
	add("Mortal Engines", "Philip Reeve")
	mort := add("Mort", "Terry Pratchett")

	cases := []struct {
		title, author string
		want          int64
		why           string
	}{
		{"Hostile Hospital", "Snicket, Lemony", hospital, "leading The is optional, author is Last, First"},
		{"Reptile Room", "Snicket, Lemony", reptile, "the author picks between two books of one title"},
		{"Reptile Room", "", 0, "two books and no author to choose: say nothing"},
		{"The Lost Files: Six's Legacy", "Pittacus Lore", six, "the part after the colon is the title"},
		{"Six's Legacy", "Pittacus Lore", six, "an exact match still works"},
		{"Hunted", "Pittacus Lore", lost, "the part after the colon in the library's title"},
		{"Mort", "Terry Pratchett", mort, "Mort is Mort, not Mortal Engines"},
		{"Mortal", "", 0, "a prefix is not a title"},
		{"Dune", "Frank Herbert", 0, "not in the library"},
		{"ERROR: Error reading EPUB format", "N/A", 0, "junk stays unmatched"},
	}
	for _, c := range cases {
		if got := st.MatchBook(c.title, c.author); got != c.want {
			t.Errorf("%s / %s = %d, want %d (%s)", c.title, c.author, got, c.want, c.why)
		}
	}
}
