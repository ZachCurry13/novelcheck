package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// fakeCalibre mimics the calibre Content server's library-info, list and
// remove endpoints (no login). titles is what calibre believes the ids are.
func fakeCalibre(t *testing.T, titles map[string]string, removed *atomic.Int32) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/ajax/library-info":
			fmt.Fprint(w, `{"library_map":{"books":"books"},"default_library":"books"}`)
		case r.URL.Path == "/cdb/cmd/list/0":
			b, _ := json.Marshal(map[string]any{"result": map[string]any{"data": map[string]any{"title": titles}}})
			w.Write(b)
		case r.URL.Path == "/cdb/cmd/remove/0":
			var args []json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&args)
			var ids []int
			_ = json.Unmarshal(args[0], &ids)
			if string(args[1]) != "false" {
				t.Errorf("removal must go to the recycle bin, got permanent=%s", args[1])
			}
			removed.Add(int32(len(ids)))
			fmt.Fprint(w, `{"result":null}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestOneClickCalibreRemoval(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	for i, title := range []string{"Spicy One", "Spicy Two"} {
		id, _ := st.UpsertBook(title, "Author", "", "")
		_ = st.AddCopy(cal, id, "", "epub", fmt.Sprint(i+1))
		_ = st.SaveAnalysis(id, store.Analysis{Classification: "Open Door"})
	}
	body := map[string]any{"hide": "open_door", "expected_count": 2}

	if res, _ := admin.do("POST", "/api/admin/calibre/remove", body, true); res.StatusCode != 400 {
		t.Fatalf("removal without a Content server should be refused: %d", res.StatusCode)
	}

	// Wrong library: calibre's book #2 is a different title -> nothing removed.
	var removed atomic.Int32
	wrong := fakeCalibre(t, map[string]string{"1": "Spicy One", "2": "Grandma's Cookbook"}, &removed)
	if res, out := admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": wrong.URL}, true); res.StatusCode != 200 {
		t.Fatalf("save server: %d %v", res.StatusCode, out)
	}
	res, out := admin.do("POST", "/api/admin/calibre/remove", body, true)
	if res.StatusCode != 409 || !strings.Contains(out["error"].(string), "Nothing was removed") || removed.Load() != 0 {
		t.Fatalf("title mismatch must stop everything: %d %v removed=%d", res.StatusCode, out, removed.Load())
	}

	// List changed since review -> refused.
	right := fakeCalibre(t, map[string]string{"1": "Spicy One", "2": "Spicy Two"}, &removed)
	admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": right.URL}, true)
	if res, _ := admin.do("POST", "/api/admin/calibre/remove", map[string]any{"hide": "open_door", "expected_count": 5}, true); res.StatusCode != 409 {
		t.Fatalf("stale count must be refused: %d", res.StatusCode)
	}
	res, out = admin.do("POST", "/api/admin/calibre/remove", body, true)
	if res.StatusCode != 200 || out["removed"].(float64) != 2 || removed.Load() != 2 {
		t.Fatalf("removal: %d %v removed=%d", res.StatusCode, out, removed.Load())
	}

	// Password is never returned.
	_, cfg := admin.do("GET", "/api/admin/calibre/server", nil, false)
	if _, ok := cfg["password"]; ok || cfg["configured"] != true {
		t.Fatalf("server config leaked or wrong: %v", cfg)
	}
}
