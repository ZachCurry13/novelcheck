package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// A Calibre that refuses one book ("Foreign key violation") must not stop the rest, the refused book is named, and
// books its Content server doesn't know yet (files on disk) are counted as such, not as "already gone".
func TestRemovalSurvivesOneRefusedBookAndNamesTheLaggingOnes(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	dir := t.TempDir()
	id, _ := st.UpsertBook("Same", "Author", "", "")
	for _, c := range []struct {
		ext    string
		exists bool
	}{{"1", true}, {"2", true}, {"3", true}, {"4", true}, {"5", false}} { // #1 stays; #4 isn't on the Content server yet; #5 is gone
		p := filepath.Join(dir, c.ext+".epub")
		if c.exists {
			_ = os.WriteFile(p, []byte("x"), 0o644)
		}
		_ = st.AddCopy(cal, id, p, "epub", c.ext)
	}

	var removedCalls atomic.Int32
	cs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ajax/library-info":
			fmt.Fprint(w, `{"library_map":{"books":"books"},"default_library":"books"}`)
		case "/cdb/cmd/list/0":
			b, _ := json.Marshal(map[string]any{"result": map[string]any{"data": map[string]any{"title": map[string]string{"1": "Same", "2": "Same", "3": "Same"}}}})
			w.Write(b) // #4 isn't known yet; #5 is gone
		case "/cdb/cmd/remove/0":
			var args []json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&args)
			var ids []int
			_ = json.Unmarshal(args[0], &ids)
			removedCalls.Add(1)
			for _, id := range ids {
				if id == 3 {
					fmt.Fprint(w, `{"err":"Foreign key violation: authors is still referenced"}`)
					return
				}
			}
			fmt.Fprint(w, `{"result":null}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer cs.Close()
	admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": cs.URL}, true)

	// the duplicates page's group is needed first (it builds from the library)
	res, out := admin.do("POST", "/api/admin/calibre/duplicates/remove", map[string]any{"remove": []string{"2", "3", "4", "5"}, "later": true}, true)
	if res.StatusCode != 200 {
		t.Fatalf("a partly refused removal still answers: %d %v", res.StatusCode, out)
	}
	if out["removed"] != 1.0 || out["behind"] != 1.0 || out["skipped"] != 2.0 {
		t.Fatalf("one removed, one behind, one gone: %v", out)
	}
	if f, _ := out["failed"].([]any); len(f) != 1 || !strings.Contains(fmt.Sprint(f[0]), "Foreign key violation") {
		t.Fatalf("the refused book is named: %v", out["failed"])
	}
	if log := st.CleanupLog(3); len(log) == 0 || !strings.Contains(log[0].Text, "behind") {
		t.Fatalf("it is in the cleanup log: %+v", log)
	}
}
