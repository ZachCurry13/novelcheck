package api_test

import (
	"fmt"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestLibraryBrowseFilters(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	kindle, _ := st.EnsureCatalog("My Kindle", "drive")
	for i, b := range []struct {
		title, author, series string
		tags                  []string
		cat                   int64
	}{
		{"Guards! Guards! (Discworld, #8)", "Terry Pratchett", "", []string{"Fiction", "Fantasy", "Humor"}, cal},
		{"Men at Arms (Discworld, #9)", "Terry Pratchett", "", []string{"Fantasy"}, kindle},
		{"Good Omens", "Terry Pratchett & Neil Gaiman", "", []string{"Fantasy", "Comedy"}, cal},
		{"Team of Rivals", "Doris Kearns Goodwin", "", []string{"Nonfiction", "History", "Biography"}, cal},
		{"Untagged Thing", "Someone", "", nil, cal},
	} {
		id, _ := st.UpsertCalibreBook(b.cat, store.CalibreEntry{ExtID: fmt.Sprint(i + 1), Title: b.title, Authors: b.author, Tags: b.tags})
		_ = st.AddCalibreCopy(b.cat, id, fmt.Sprintf("/calibre/%d.epub", i), "epub", fmt.Sprint(i+1), "")
	}
	total := func(q string) float64 {
		t.Helper()
		_, out := admin.do("GET", "/api/books?"+q, nil, false)
		return out["total"].(float64)
	}
	if total("genre=fantasy") != 3 || total("genre=history") != 1 || total("genre=bogus") != 5 {
		t.Fatal("genre filter")
	}
	if total("kind=fiction") != 3 || total("kind=nonfiction") != 1 || total("kind=unknown") != 1 {
		t.Fatal("fiction / nonfiction filter")
	}
	if total("author=Gaiman") != 1 || total("author=Pratchett") != 3 || total("series=discworld") != 2 || total("q=biography") != 1 {
		t.Fatal("author, series and tag search")
	}

	_, f := admin.do("GET", "/api/books/facets", nil, false)
	g := f["genres"].([]any)[0].(map[string]any)
	authors := f["authors"].([]any)
	if g["key"] != "fantasy" || g["count"].(float64) != 3 || authors[0] != "Terry Pratchett" || len(f["series"].([]any)) != 1 {
		t.Fatalf("filter choices: %v", f)
	}

	// Suggested Reads can come from one library only.
	_ = st.SetSetting(store.KeySuggestMode, store.SuggestFree)
	var guards int64
	_ = st.DB.Get(&guards, `SELECT id FROM books WHERE title = 'Guards! Guards!'`)
	admin.do("POST", "/api/queue", map[string]int64{"book_id": guards}, true)
	titles := func(q string) map[string]bool {
		_, out := admin.do("GET", "/api/suggestions"+q, nil, false)
		m := map[string]bool{}
		for _, it := range out["items"].([]any) {
			m[it.(map[string]any)["book"].(map[string]any)["title"].(string)] = true
		}
		return m
	}
	if all := titles(""); !all["Men at Arms"] || !all["Good Omens"] {
		t.Fatalf("every library: %v", all)
	}
	if mine := titles(fmt.Sprintf("?catalog=%d", kindle)); len(mine) != 1 || !mine["Men at Arms"] {
		t.Fatalf("just the Kindle: %v", mine)
	}
}
