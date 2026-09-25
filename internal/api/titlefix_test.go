package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestTidyTitlesInCalibre(t *testing.T) {
	var mu sync.Mutex
	calibre := map[string]string{"7": "01 - Guards! Guards!", "8": "02 - Mort"}
	var saved []string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/ajax/library-info":
			fmt.Fprint(w, `{"library_map":{"books":"books"},"default_library":"books"}`)
		case "/cdb/cmd/list/0":
			b, _ := json.Marshal(map[string]any{"result": map[string]any{"data": map[string]any{"title": calibre}}})
			w.Write(b)
		case "/cdb/cmd/set_metadata/0":
			var args []json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&args)
			saved = append(saved, string(args[1])+" "+string(args[2]))
			fmt.Fprint(w, `{"result":{"!_":2,"!v":{}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer fake.Close()
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	ids := map[string]int64{}
	for ext, title := range calibre {
		id, _ := st.UpsertCalibreBook(cat, store.CalibreEntry{ExtID: ext, Title: title, Authors: "Terry Pratchett"})
		_ = st.AddCalibreCopy(cat, id, "/calibre/"+ext+".epub", "epub", ext, "")
		ids[ext] = id
	}

	_, out := admin.do("GET", "/api/admin/title-fixes", nil, false)
	if fixes := out["fixes"].([]any); len(fixes) != 2 || out["server"] != false {
		t.Fatalf("list: %v", out)
	}
	if res, _ := admin.do("POST", "/api/admin/title-fixes", map[string]any{"ids": []int64{ids["7"]}}, true); res.StatusCode != 400 {
		t.Fatalf("no Content server yet: %d", res.StatusCode)
	}
	if res, _ := admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": fake.URL}, true); res.StatusCode != 200 {
		t.Fatal("save server")
	}

	// One click: the tidy title goes to Calibre.
	if res, out := admin.do("POST", "/api/admin/title-fixes", map[string]any{"ids": []int64{ids["7"]}}, true); res.StatusCode != 200 || out["fixed"].(float64) != 1 {
		t.Fatalf("fix: %d %v", res.StatusCode, out)
	}
	if len(saved) != 1 || saved[0] != `7 [["title","Guards! Guards!"]]` || st.CountTitleFixes() != 1 {
		t.Fatalf("sent to calibre: %v", saved)
	}

	// Edited by hand, with a series.
	body := map[string]any{"title": "Mort", "series": "Discworld", "series_index": 4}
	res, out := admin.do("POST", fmt.Sprintf("/api/books/%d/calibre-title", ids["8"]), body, true)
	if book, _ := out["book"].(map[string]any); res.StatusCode != 200 || book["series"] != "Discworld" || book["title_fix"] != "" {
		t.Fatalf("save: %d %v", res.StatusCode, out)
	}
	if saved[1] != `8 [["title","Mort"],["series","Discworld"],["series_index",4]]` {
		t.Fatalf("with series: %v", saved)
	}

	// A different library behind the Content server: nothing is changed.
	mu.Lock()
	calibre["7"] = "Grandma's Cookbook"
	mu.Unlock()
	res, out = admin.do("POST", fmt.Sprintf("/api/books/%d/calibre-title", ids["7"]), map[string]any{"title": "Guards! Guards!"}, true)
	if res.StatusCode != 409 || !strings.Contains(out["error"].(string), "Nothing was changed") || len(saved) != 2 {
		t.Fatalf("wrong library: %d %v", res.StatusCode, out)
	}

	// Admins only; titles are required.
	if res, _ := admin.do("POST", fmt.Sprintf("/api/books/%d/calibre-title", ids["8"]), map[string]any{"title": " "}, true); res.StatusCode != 400 {
		t.Fatal("empty title")
	}
	_, _ = admin.do("POST", "/api/admin/users", map[string]any{"username": "ed", "password": "editorpass1", "role": "editor"}, true)
	editor := login(t, srv, "ed", "editorpass1")
	if res, _ := editor.do("GET", "/api/admin/title-fixes", nil, false); res.StatusCode != 403 {
		t.Fatalf("editors can't change Calibre: %d", res.StatusCode)
	}
}
