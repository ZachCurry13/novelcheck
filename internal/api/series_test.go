package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// A series in order, with the numbers the family doesn't have, what the
// reader has read, and the next book.
func TestSeriesPage(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	u, _ := st.UserByName("admin")
	ids := map[string]int64{}
	for _, b := range []struct {
		title string
		n     float64
	}{{"The Colour of Magic", 1}, {"The Light Fantastic", 2}, {"Mort", 4}, {"Sourcery", 5}, {"Guards! Guards!", 8}} {
		id, _ := st.UpsertBook(b.title, "Terry Pratchett", "", "")
		_ = st.AddCopy(cal, id, "/c/"+b.title+".epub", "epub", b.title)
		_, _ = st.DB.Exec(`UPDATE books SET series = 'Discworld', series_index = ? WHERE id = ?`, b.n, id)
		ids[b.title] = id
	}
	// Only listed on Discover: not the family's, so not in the series.
	disc, _ := st.EnsureCatalog(store.DiscoverCatalog, "custom")
	listed, _ := st.UpsertBook("Equal Rites", "Terry Pratchett", "", "")
	_ = st.AddCopy(disc, listed, "discover:1", "list", "")
	_, _ = st.DB.Exec(`UPDATE books SET series = 'Discworld', series_index = 3 WHERE id = ?`, listed)

	_ = st.Enqueue(u.ID, ids["The Colour of Magic"])
	_, _ = st.DB.Exec(`UPDATE queue_items SET status = 'finished' WHERE book_id = ?`, ids["The Colour of Magic"])
	_ = st.Enqueue(u.ID, ids["The Light Fantastic"])
	_, _ = st.DB.Exec(`UPDATE queue_items SET status = 'finished' WHERE book_id = ?`, ids["The Light Fantastic"])

	res, list := admin.doList("GET", "/api/series")
	if res.StatusCode != 200 || len(list) != 1 || list[0]["name"] != "Discworld" || list[0]["books"].(float64) != 5 || list[0]["read"].(float64) != 2 {
		t.Fatalf("list: %d %v", res.StatusCode, list)
	}
	_, one := admin.do("GET", "/api/series/one?name=discworld", nil, false)
	books := one["books"].([]any)
	gaps := one["gaps"].([]any)
	if len(books) != 5 || books[2].(map[string]any)["title"] != "Mort" || len(gaps) != 3 || gaps[0].(float64) != 3 || gaps[2].(float64) != 7 ||
		int64(one["next"].(float64)) != ids["Mort"] {
		t.Fatalf("one: %d books, gaps %v, next %v", len(books), gaps, one["next"])
	}
	if res, _ := admin.do("GET", "/api/series/one?name=Nope", nil, false); res.StatusCode != 404 {
		t.Fatalf("unknown series: %d", res.StatusCode)
	}
}
