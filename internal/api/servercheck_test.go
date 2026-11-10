package api_test

import (
	"io"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// "Check my servers": read-only, says what Calibre has, and the answer appears in the server report.
func TestServerChecks(t *testing.T) {
	cal := &fakeCalibreDetails{book: map[string]any{"title": "Cinder", "authors": "Marissa Meyer", "series": "", "series_index": 1.0,
		"tags": []string{}, "comments": "", "identifiers": map[string]string{}}}
	cs := cal.serve(t)
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	catalog, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	id, _ := st.UpsertBook("Cinder", "Marissa Meyer", "", "")
	_ = st.AddCopy(catalog, id, "/c/cinder.epub", "epub", "5")
	admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": cs.URL}, true)

	_, out := admin.do("POST", "/api/admin/server-checks", nil, true)
	text, _ := out["text"].(string)
	for _, want := range []string{"libraries it serves: books = “books”", "reading a book's details: works", "route for setting a cover: not available", "== Audiobookshelf ==\nnot set up"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if len(cal.sets) != 0 {
		t.Fatalf("a check never writes: %v", cal.sets)
	}
	res, err := admin.http.Get(admin.base + "/api/admin/diagnostics")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(body), "Server checks") || !strings.Contains(string(body), "libraries it serves") {
		t.Fatalf("the report carries the last check:\n%s", body)
	}
}
