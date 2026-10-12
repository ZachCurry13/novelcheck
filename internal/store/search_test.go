package store_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestSearchMatchesEachWord(t *testing.T) {
	st := newStore(t)
	a, _ := st.UpsertBook("The Reptile Room", "Lemony Snicket", "", "")
	b, _ := st.UpsertBook("Nine's Legacy", "Pittacus Lore", "", "")
	_, _ = st.UpsertBook("Mort", "Terry Pratchett", "", "")
	find := func(q string) []int64 {
		books, _, err := st.ListBooks(store.BookFilter{Query: q, Limit: 20}, nil)
		if err != nil {
			t.Fatal(err)
		}
		var ids []int64
		for _, bk := range books {
			ids = append(ids, bk.ID)
		}
		return ids
	}
	for q, want := range map[string]int64{
		"reptile snicket":   a, // words from the title and the author
		"snicket reptile":   a, // in any order
		"The Reptile Room":  a,
		"REPTILE":           a,
		"nines legacy":      b, // no apostrophe
		"Nine's Legacy":     b,
		"lore nine":         b,
		"reptile, snicket!": a, // punctuation
	} {
		if got := find(q); len(got) != 1 || got[0] != want {
			t.Errorf("%q found %v, want [%d]", q, got, want)
		}
	}
	if got := find("reptile pratchett"); len(got) != 0 {
		t.Errorf("every word must match: %v", got)
	}
}
