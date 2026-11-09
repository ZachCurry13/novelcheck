package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// "Find the missing books": the numbers of a series the family lacks are filled in from Hardcover as looked-up
// books in their place, so they show on the Series page and can be requested or found at a library.
func TestSeriesFill(t *testing.T) {
	hc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"series":[{"name":"The Lunar Chronicles","book_series":[
		 {"position":1,"book":{"title":"Cinder","release_year":2012,"cached_image":{"url":"https://img.example/cinder.jpg"},"cached_contributors":[{"author":{"name":"Marissa Meyer"}}]}},
		 {"position":2,"book":{"title":"Scarlet","release_year":2013,"cached_image":{"url":"https://img.example/scarlet.jpg"},"cached_contributors":[{"author":{"name":"Marissa Meyer"}}]}},
		 {"position":3,"book":{"title":"Cress","release_year":2014,"cached_image":{},"cached_contributors":[]}},
		 {"position":3.5,"book":{"title":"Fairest","release_year":2015,"cached_image":{},"cached_contributors":[{"author":{"name":"Marissa Meyer"}}]}},
		 {"position":4,"book":{"title":"Winter","release_year":2015,"cached_image":{},"cached_contributors":[{"author":{"name":"Marissa Meyer"}}]}}]}]}}`)
	}))
	defer hc.Close()
	api.SetHardcoverURL(hc.URL)
	t.Cleanup(func() { api.SetHardcoverURL("") })
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	u, _ := st.UserByName("admin")
	for _, b := range []struct {
		t string
		n float64
	}{{"Cinder", 1}, {"Cress", 3}} {
		id, _ := st.UpsertBook(b.t, "Marissa Meyer", "", "")
		_ = st.AddCopy(cal, id, "/c/"+b.t+".epub", "epub", b.t)
		st.DB.MustExec(`UPDATE books SET series = 'The Lunar Chronicles', series_index = ? WHERE id = ?`, b.n, id)
	}
	if res, _ := admin.do("POST", "/api/series/fill?name=The%20Lunar%20Chronicles", map[string]string{}, true); res.StatusCode != 400 {
		t.Fatalf("no Hardcover token yet: %d", res.StatusCode)
	}
	_ = st.SetHardcoverToken(u.ID, "tok")
	res, out := admin.do("POST", "/api/series/fill?name=The%20Lunar%20Chronicles", map[string]string{}, true)
	if res.StatusCode != 200 || out["found"] != float64(5) || out["added"] != float64(3) {
		t.Fatalf("fill: %d %v", res.StatusCode, out)
	}
	_, one := admin.do("GET", "/api/series/one?name=The%20Lunar%20Chronicles", nil, false)
	books, _ := one["books"].([]any)
	if len(books) != 5 {
		t.Fatalf("the family's two and the three that were missing: %d", len(books))
	}
	if gaps, _ := one["gaps"].([]any); len(gaps) != 0 {
		t.Fatalf("no number is missing any more: %v", gaps)
	}
	// Filling again adds nothing: the missing books are there now.
	if _, again := admin.do("POST", "/api/series/fill?name=The%20Lunar%20Chronicles", map[string]string{}, true); again["added"] != float64(0) {
		t.Fatalf("again: %v", again)
	}
	// A filled-in book is a looked-up one: not in a library, with its cover address kept.
	var owned bool
	var cover string
	st.DB.Get(&cover, `SELECT cover_url FROM books WHERE title = 'Scarlet'`)
	for _, b := range books {
		m := b.(map[string]any)
		if m["title"] == "Scarlet" {
			owned, _ = m["owned"].(bool)
		}
	}
	if owned || cover != "https://img.example/scarlet.jpg" {
		t.Fatalf("Scarlet: owned=%v cover=%q", owned, cover)
	}
}
