package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// A Calibre title with a typo and dots ("Unnatural.Exprosure", no ISBN) still finds its book on Hardcover:
// the true title comes from the author's books, and the fix to the title itself is offered.
func TestHardcoverScanFindsATypoTitle(t *testing.T) {
	hc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var q struct{ Query string }
		_ = json.Unmarshal(raw, &q)
		switch {
		case strings.Contains(q.Query, `search(query: "Unnatural Exposure Patricia Cornwell"`):
			io.WriteString(w, `{"data": {"search": {"ids": [44]}}}`)
		case strings.Contains(q.Query, "search("):
			io.WriteString(w, `{"data": {"search": {"ids": []}}}`)
		case strings.Contains(q.Query, "_in: [44]"):
			io.WriteString(w, `{"data": {"books": [{"id": 44, "title": "Unnatural Exposure", "release_year": 1997,
				"contributions": [{"author": {"name": "Patricia Cornwell"}}], "editions": []}]}}`)
		default:
			io.WriteString(w, `{"data": {"me": [{"id": 7, "username": "reader7"}]}}`)
		}
	}))
	defer hc.Close()
	api.SetHardcoverURL(hc.URL)
	t.Cleanup(func() { api.SetHardcoverURL("") })
	ol := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		docs := []any{}
		if strings.Contains(r.URL.Query().Get("author"), "Cornwell") {
			docs = append(docs, map[string]any{"title": "Unnatural Exposure", "author_name": []string{"Patricia Cornwell"}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"docs": docs})
	}))
	defer ol.Close()
	cal := &fakeCalibreDetails{book: map[string]any{"title": "Unnatural.Exprosure", "authors": "Patricia Cornwell", "series": "",
		"series_index": 1.0, "tags": []string{}, "comments": "", "identifiers": map[string]string{}, "cover": ""}}
	cs := cal.serve(t)
	srv, st := setupWith(t, func(s *api.Server) {
		s.Worker.NewEnricher = func() *enrich.Client {
			c := enrich.New("")
			c.OpenLibraryURL, c.GoogleBooksURL = ol.URL, ol.URL
			return c
		}
	})
	admin := login(t, srv, "admin", "adminpass1")
	catalog, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	id, _ := st.UpsertBook("Unnatural.Exprosure", "Patricia Cornwell", "", "")
	_ = st.AddCopy(catalog, id, "/c/u.epub", "epub", "5")
	admin.do("PUT", "/api/me/hardcover", map[string]string{"token": "good-token"}, true)
	admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": cs.URL}, true)
	if res, out := admin.do("POST", "/api/admin/hc-clean/scan", nil, true); res.StatusCode != 202 {
		t.Fatalf("scan: %d %v", res.StatusCode, out)
	}
	state := waitHC(t, admin)
	for _, s := range state["suggestions"].([]any) {
		if m := s.(map[string]any); m["field"] == "title" && strings.Contains(m["shown"].(string), "Unnatural Exposure") {
			return
		}
	}
	t.Fatalf("no title fix for the typo: %v", state["suggestions"])
}
