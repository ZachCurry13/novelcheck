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

// A Calibre book with no ISBN that Hardcover doesn't know gets Open Library's ISBN offered as a best guess, by the
// same author only.
func TestHardcoverScanOffersBestGuessISBN(t *testing.T) {
	hc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), "search(") {
			io.WriteString(w, `{"data": {"search": {"ids": []}}}`)
			return
		}
		io.WriteString(w, `{"data": {"me": [{"id": 7, "username": "reader7"}]}}`)
	}))
	defer hc.Close()
	api.SetHardcoverURL(hc.URL)
	t.Cleanup(func() { api.SetHardcoverURL("") })
	ol := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		docs := []any{}
		if strings.Contains(r.URL.Query().Get("author"), "Ippolito") {
			docs = append(docs, map[string]any{"title": "Frogs at Noontide", "author_name": []string{"Janeen Ippolito"}, "isbn": []string{"1234567890", "9781234567897"}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"docs": docs})
	}))
	defer ol.Close()
	cal := &fakeCalibreDetails{book: map[string]any{"title": "Frogs At Noontide: A Quirky YA Frog Prince Retelling", "authors": "Janeen Ippolito", "series": "",
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
	id, _ := st.UpsertBook("Frogs At Noontide: A Quirky YA Frog Prince Retelling", "Janeen Ippolito", "", "")
	_ = st.AddCopy(catalog, id, "/c/f.epub", "epub", "5")
	admin.do("PUT", "/api/me/hardcover", map[string]string{"token": "good-token"}, true)
	admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": cs.URL}, true)
	if res, out := admin.do("POST", "/api/admin/hc-clean/scan", nil, true); res.StatusCode != 202 {
		t.Fatalf("scan: %d %v", res.StatusCode, out)
	}
	state := waitHC(t, admin)
	if state["no_isbn"] != 1.0 {
		t.Fatalf("books with no ISBN: %v", state["no_isbn"])
	}
	for _, s := range state["suggestions"].([]any) {
		if m := s.(map[string]any); m["field"] == "identifiers" && m["confidence"] == "best guess" && strings.Contains(m["shown"].(string), "9781234567897") {
			return
		}
	}
	t.Fatalf("no best-guess ISBN: %v", state["suggestions"])
}
