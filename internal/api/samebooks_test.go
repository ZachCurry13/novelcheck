package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// A book on someone's list before its e-book arrived (so no author to match
// on) joins the library copy by itself; ambiguous pairs wait for a person.
func TestSameBooksJoin(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	look, _ := st.EnsureCatalog(store.LookedUpCatalog, "custom")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")

	listed, _ := st.UpsertBook("Cinder", "", "", "") // wished for without an author
	_ = st.AddCopy(look, listed, "lookup:cinder", "list", "")
	admin.do("POST", "/api/queue", map[string]int64{"book_id": listed}, true)
	if _, out := admin.do("POST", "/api/books/"+itoa(listed)+"/wish", map[string]string{}, true); out == nil {
		t.Fatal("wish")
	}
	real, _ := st.UpsertBook("Cinder (The Lunar Chronicles #1)", "Meyer, Marissa", "", "")
	_ = st.AddCopy(cal, real, "/c/cinder.epub", "epub", "7")
	if listed == real {
		t.Fatal("the test needs two separate entries")
	}

	_, out := admin.do("GET", "/api/admin/same-books", nil, false)
	if pairs, _ := out["pairs"].([]any); len(pairs) != 1 {
		t.Fatalf("one pair is offered: %v", out)
	}
	st.TidySame()
	_, out = admin.do("GET", "/api/admin/same-books", nil, false)
	if pairs, _ := out["pairs"].([]any); len(pairs) != 0 {
		t.Fatalf("the clear case is joined: %v", out)
	}
	if _, err := st.BookByID(listed, nil); err == nil && listed != real {
		if _, err2 := st.BookByID(real, nil); err2 == nil {
			t.Fatal("one of the two entries is gone")
		}
	}
	if _, items := admin.doList("GET", "/api/queue"); len(items) != 1 {
		t.Fatalf("the Nightstand entry followed the book: %v", items)
	}

	// two unowned entries stay for a person; "not the same" hides them
	a, _ := st.UpsertBook("Dust", "", "", "")
	b, _ := st.UpsertBook("Dust", "Hugh Howey", "", "")
	_ = st.AddCopy(look, a, "lookup:a", "list", "")
	_ = st.AddCopy(look, b, "lookup:b", "list", "")
	st.TidySame()
	_, out = admin.do("GET", "/api/admin/same-books", nil, false)
	if pairs, _ := out["pairs"].([]any); len(pairs) != 1 {
		t.Fatalf("unowned pairs wait for a person: %v", out)
	}
	if res, _ := admin.do("POST", "/api/admin/same-books/not", map[string]int64{"a": a, "b": b}, true); res.StatusCode != 200 {
		t.Fatal("not the same")
	}
	_, out = admin.do("GET", "/api/admin/same-books", nil, false)
	if pairs, _ := out["pairs"].([]any); len(pairs) != 0 {
		t.Fatalf("hidden for good: %v", out)
	}
}

// A book and its spin-offs are different books. Only the clearest pairs are called "sure"; a
// plain title next to "Title: Subtitle" is offered with its evidence ("check"), two spin-offs
// are never paired, and clear contradictions (another number in the same series, descriptions
// about different things) take a pair off the list.
func TestSameBooksNotSpinoffs(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	look, _ := st.EnsureCatalog(store.LookedUpCatalog, "custom")
	ids := map[string]int64{}
	for _, b := range [][2]string{
		{"I Am Number Four", "Pittacus Lore"},
		{"I Am Number Four: The Lost Files: The Last Days of Lorien", "Pittacus Lore"},
		{"I Am Number Four : The Lost Files Six's Legacy", "Pittacus Lore"},
		{"Avatar, The Last Airbender: The Shadow of Kyoshi", "F. C. Yee"},
		{"Avatar, the Last Airbender", "F. C. Yee"},
		{"The Wide Window", "Lemony Snicket"},
		{"The Wide Window: Or, Disappearance!", "Lemony Snicket"},
	} {
		id, _ := st.UpsertBook(b[0], b[1], "", "")
		_ = st.AddCopy(look, id, "lookup:"+b[0], "list", "")
		ids[b[0]] = id
	}
	pairs := func() []any {
		_, out := admin.do("GET", "/api/admin/same-books", nil, false)
		p, _ := out["pairs"].([]any)
		return p
	}
	conf := func(p any) string { return p.(map[string]any)["confidence"].(string) }
	ps := pairs()
	if len(ps) != 4 { // Wide Window; the plain title with each of its two spin-offs; Avatar with Shadow of Kyoshi
		t.Fatalf("pairs: %d %v", len(ps), ps)
	}
	if conf(ps[0]) != "sure" || ps[0].(map[string]any)["a"].(map[string]any)["title"] != "The Wide Window" {
		t.Fatalf("the sure one comes first: %v", ps[0])
	}
	for _, p := range ps[1:] {
		if conf(p) != "check" {
			t.Fatalf("a subtitle pair with no evidence needs a look: %v", p)
		}
	}
	if len(st.AutoMergeSame()) != 0 {
		t.Fatal("nothing here is in a library, and subtitle pairs are never joined by themselves")
	}
	// Book 1 and book 5 of a series are different books: that pair leaves the list.
	four, five := ids["I Am Number Four"], ids["I Am Number Four: The Lost Files: The Last Days of Lorien"]
	st.DB.MustExec(`UPDATE books SET series = 'Lorien Legacies', series_index = 1 WHERE id = ?`, four)
	st.DB.MustExec(`UPDATE books SET series = 'Lorien Legacies', series_index = 5 WHERE id = ?`, five)
	if got := len(pairs()); got != 3 {
		t.Fatalf("different series numbers settle it: %d pairs", got)
	}
	// Descriptions that say the same thing make a subtitle pair "likely".
	text := "A boy hides on Earth with his guardian from the Mogadorians who destroyed his home planet Lorien and hunt the survivors one by one."
	shadow, plain := ids["Avatar, The Last Airbender: The Shadow of Kyoshi"], ids["Avatar, the Last Airbender"]
	st.DB.MustExec(`UPDATE books SET description = ? WHERE id IN (?, ?)`, text, shadow, plain)
	found := false
	for _, p := range pairs() {
		if p.(map[string]any)["a"].(map[string]any)["id"] == float64(shadow) || p.(map[string]any)["b"].(map[string]any)["id"] == float64(shadow) {
			found = conf(p) == "likely"
		}
	}
	if !found {
		t.Fatal("matching descriptions make the pair likely")
	}
	// Descriptions about different things take the pair off.
	st.DB.MustExec(`UPDATE books SET description = 'Zebras gallop across the Serengeti plains while rival herds migrate toward distant rivers and scarce grazing land.' WHERE id = ?`, plain)
	for _, p := range pairs() {
		if p.(map[string]any)["a"].(map[string]any)["id"] == float64(shadow) {
			t.Fatalf("different descriptions rule it out: %v", p)
		}
	}
}
