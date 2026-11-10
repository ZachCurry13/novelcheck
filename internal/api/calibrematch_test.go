package api_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The Content server's default library is not the one NovelCheck reads: Tidy finds the library whose book
// numbers and titles match, switches to it, and tidies there.
func TestTidyFindsTheRightServerLibrary(t *testing.T) {
	libs := map[string]map[string]string{
		"config": {"5": "Something Else"},
		"clean":  {"1690": "Scarlet (The Lunar Chronicles Book 2)"},
	}
	var savedIn []string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lib := r.URL.Query().Get("library_id")
		if lib == "" {
			lib = "config"
		}
		switch r.URL.Path {
		case "/ajax/library-info":
			fmt.Fprint(w, `{"library_map":{"config":"config","clean":"Clean Library"},"default_library":"config"}`)
		case "/cdb/cmd/list/0":
			raw, _ := io.ReadAll(r.Body)
			asked := map[string]string{} // the real server answers only for the numbers asked
			for id, title := range libs[lib] {
				if strings.Contains(string(raw), "id:"+id) {
					asked[id] = title
				}
			}
			b, _ := json.Marshal(map[string]any{"result": map[string]any{"data": map[string]any{"title": asked}}})
			w.Write(b)
		case "/cdb/cmd/set_metadata/0":
			savedIn = append(savedIn, lib)
			fmt.Fprint(w, `{"result":{"!_":2,"!v":{}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer fake.Close()
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	id, _ := st.UpsertCalibreBook(cat, store.CalibreEntry{ExtID: "1690", Title: "Scarlet (The Lunar Chronicles Book 2)", Authors: "Marissa Meyer"})
	_ = st.AddCalibreCopy(cat, id, "/calibre/1690.epub", "epub", "1690", "")
	if res, _ := admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": fake.URL}, true); res.StatusCode != 200 {
		t.Fatal("save server")
	}
	if st.Setting(store.KeyCalibreSrvLibrary) != "config" {
		t.Fatalf("starts on the server's default: %q", st.Setting(store.KeyCalibreSrvLibrary))
	}
	res, out := admin.do("POST", "/api/admin/title-fixes", map[string]any{"ids": []int64{id}}, true)
	if res.StatusCode != 200 || out["fixed"].(float64) != 1 {
		t.Fatalf("tidied after switching: %d %v", res.StatusCode, out)
	}
	if len(savedIn) != 1 || savedIn[0] != "clean" || st.Setting(store.KeyCalibreSrvLibrary) != "clean" {
		t.Fatalf("it wrote to and remembered the matching library: %v %q", savedIn, st.Setting(store.KeyCalibreSrvLibrary))
	}

	// Nothing matches anywhere: the skip says what each library holds.
	libs["clean"] = map[string]string{}
	other, _ := st.UpsertCalibreBook(cat, store.CalibreEntry{ExtID: "77", Title: "02 - Lost Book", Authors: "Nobody"})
	_ = st.AddCalibreCopy(cat, other, "/calibre/77.epub", "epub", "77", "")
	_, out = admin.do("POST", "/api/admin/title-fixes", map[string]any{"ids": []int64{other}}, true)
	skipped, _ := out["skipped"].([]any)
	if len(skipped) != 1 || !strings.Contains(skipped[0].(string), "What the Content server serves") {
		t.Fatalf("explains what it serves: %v", out)
	}
}
