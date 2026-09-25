package suggest

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func book(id int64, title, author, series string, idx float64, desc string) store.SuggestBook {
	return store.SuggestBook{ID: id, Title: title, Author: author, Series: series, SeriesIndex: idx, Description: desc, Spice: 1}
}

var pool = []store.SuggestBook{
	book(1, "Men at Arms", "Terry Pratchett", "Discworld", 9, "The city watch of Ankh-Morpork hunts a killer with a strange weapon."),
	book(2, "Eric", "Terry Pratchett", "Discworld", 10, "A demonologist summons a wizard by mistake."),
	book(3, "Good Omens", "Terry Pratchett & Neil Gaiman", "", 0, "An angel and a demon try to stop the apocalypse."),
	book(4, "Dragonflight", "Anne McCaffrey", "", 0, "Riders bond with telepathic dragons to fight the deadly thread falling on Pern."),
	book(5, "Pride and Prejudice", "Jane Austen", "", 0, "Elizabeth Bennet and Mr Darcy in regency England."),
}

func signal(b store.SuggestBook, w float64) store.SuggestSignal {
	return store.SuggestSignal{SuggestBook: b, Weight: w}
}

func TestRank(t *testing.T) {
	signals := []store.SuggestSignal{
		signal(book(10, "Guards! Guards!", "Terry Pratchett", "Discworld", 8, "The night watch of Ankh-Morpork faces a dragon."), 3),
		signal(book(11, "Dragon Rider", "Cornelia Funke", "", 0, "A young silver dragon and his riders search for the rim of heaven, where dragons live safe."), 2),
	}
	picks := Rank(pool, signals, nil, 10)
	reasons := map[string]string{}
	for _, p := range picks {
		reasons[p.Book.Title] = p.Reason
	}
	if len(picks) == 0 || picks[0].Book.Title != "Men at Arms" || reasons["Men at Arms"] != "Next in Discworld after Guards! Guards!" {
		t.Fatalf("the next book in the series comes first: %+v", picks)
	}
	if _, ok := reasons["Eric"]; ok {
		t.Fatal("one book per series at a time")
	}
	if reasons["Good Omens"] != "More by Terry Pratchett" || reasons["Dragonflight"] != "Like Dragon Rider" {
		t.Fatalf("author and description matches: %v", reasons)
	}
	if _, ok := reasons["Pride and Prejudice"]; ok {
		t.Fatal("unrelated books aren't suggested")
	}

	// A 👎 on another dragon book by the same author pushes Dragonflight out.
	down := append(signals, signal(book(0, "Dragonsong", "Anne McCaffrey", "", 0, "dragons of Pern"), -1))
	for _, p := range Rank(pool, down, nil, 10) {
		if p.Book.Title == "Dragonflight" {
			t.Fatalf("similar to a 👎: %+v", p)
		}
	}
}

func TestRankWithNothingToGoOn(t *testing.T) {
	picks := Rank(pool, nil, map[int64]int{5: 2}, 10)
	if len(picks) != 1 || picks[0].Book.Title != "Pride and Prejudice" || picks[0].Reason != "Popular in your family" {
		t.Fatalf("family favourites: %+v", picks)
	}
	if len(Rank(pool, nil, nil, 10)) != 0 {
		t.Fatal("nothing to suggest yet")
	}
}

func titles(picks []Pick) map[string]string {
	out := map[string]string{}
	for _, p := range picks {
		out[p.Book.Title] = p.Reason
	}
	return out
}

func TestDislikeReasons(t *testing.T) {
	guards := signal(book(10, "Guards! Guards!", "Terry Pratchett", "Discworld", 8, "The night watch of Ankh-Morpork faces a dragon."), 3)
	down := func(b store.SuggestBook, reason string) store.SuggestSignal {
		s := signal(b, -1)
		s.Reason, s.Status = reason, "down"
		return s
	}
	// "Already read it" is reading history: the series moves on past it.
	read := signal(pool[0], 1.5) // Men at Arms, #9
	read.Status, read.Reason = "read", "read"
	if got := titles(Rank(pool[1:], []store.SuggestSignal{guards, read}, nil, 10)); got["Eric"] != "Next in Discworld after Men at Arms" {
		t.Fatalf("after a book already read: %v", got)
	}

	// "Not my kind of story" drops books described alike, not the whole author.
	dragons := signal(book(11, "Dragon Rider", "Cornelia Funke", "", 0, "A young silver dragon and his riders search for the rim of heaven, where dragons live safe."), 2)
	story := down(book(0, "Dragonsdawn", "Someone Else", "", 0, "Telepathic dragons and their riders fight the thread falling on Pern."), "story")
	if got := titles(Rank(pool, []store.SuggestSignal{guards, dragons, story}, nil, 10)); got["Dragonflight"] != "" || got["Men at Arms"] == "" {
		t.Fatalf("not my kind of story: %v", got)
	}

	// "Too spicy" drops books at that pepper level or higher.
	hot := book(6, "Night Watch", "Terry Pratchett", "Discworld", 29, "The watch again.")
	hot.Spice = 4
	spicy := down(book(0, "Something Steamy", "Anyone", "", 0, "x"), "spicy")
	spicy.Spice = 3
	if got := titles(Rank([]store.SuggestBook{hot}, []store.SuggestSignal{guards, spicy}, nil, 10)); len(got) != 0 {
		t.Fatalf("too spicy: %v", got)
	}

	// "Not this author" and "Not this series".
	author := down(book(0, "Coraline", "Neil Gaiman", "", 0, "x"), "author")
	omens := book(7, "Neverwhere", "Neil Gaiman", "", 0, "A night watch of London below.")
	if got := titles(Rank([]store.SuggestBook{omens}, []store.SuggestSignal{guards, author}, nil, 10)); len(got) != 0 {
		t.Fatalf("not this author: %v", got)
	}
	series := down(book(0, "Mort", "Terry Pratchett", "Discworld", 4, "x"), "series")
	if got := titles(Rank(pool[:2], []store.SuggestSignal{guards, series}, nil, 10)); len(got) != 0 {
		t.Fatalf("not this series: %v", got)
	}
}

func TestSample(t *testing.T) {
	got := Sample(pool, nil, map[int64]bool{5: true}, 3)
	seen := map[string]bool{}
	for _, b := range got {
		if b.ID == 5 || b.SeriesIndex > 1 && len(got) < 3 {
			t.Fatalf("excluded or later series book first: %+v", got)
		}
		seen[b.Title] = true
	}
	// Varied first (one Pratchett, no later Discworld books), then filled up.
	if len(got) != 3 || !seen["Dragonflight"] {
		t.Fatalf("sample: %+v", got)
	}
	if all := Sample(pool, nil, nil, 20); len(all) != len(pool) {
		t.Fatalf("a small library shows everything: %d", len(all))
	}
}
