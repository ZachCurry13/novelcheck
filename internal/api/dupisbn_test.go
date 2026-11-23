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

// Removing the duplicate that holds the only ISBN gives that ISBN to the copy that stays (other identifiers kept).
func TestRemovingDuplicateKeepsISBN(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	hobbit, _ := st.UpsertBook("The Hobbit", "J.R.R. Tolkien", "", "")
	_ = st.AddCopy(cal, hobbit, "/calibre/h10/h.mobi", "MOBI", "10")
	_ = st.AddCopy(cal, hobbit, "/calibre/h11/h.epub", "EPUB", "11")
	var set string
	cs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ajax/library-info":
			fmt.Fprint(w, `{"library_map":{"books":"books"},"default_library":"books"}`)
		case "/cdb/cmd/list/0":
			fmt.Fprint(w, `{"result":{"data":{"title":{"10":"The Hobbit","11":"The Hobbit"},
				"identifiers":{"10":{"isbn":"9780261103344"},"11":{"goodreads":"5907"}}}}}`)
		case "/cdb/cmd/set_metadata/0":
			b, _ := io.ReadAll(r.Body)
			set = string(b)
			fmt.Fprint(w, `{"result":{"11":{}}}`)
		case "/cdb/cmd/remove/0":
			fmt.Fprint(w, `{"result":null}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer cs.Close()
	admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": cs.URL}, true)
	if res, out := admin.do("POST", "/api/admin/calibre/duplicates/remove", map[string]any{"remove": []string{"10"}}, true); res.StatusCode != 200 {
		t.Fatalf("remove: %d %v", res.StatusCode, out)
	}
	var args []json.RawMessage
	_ = json.Unmarshal([]byte(set), &args)
	if len(args) < 3 || string(args[1]) != "11" || !strings.Contains(string(args[2]), "9780261103344") || !strings.Contains(string(args[2]), "5907") {
		t.Fatalf("the ISBN wasn't carried to #11: %s", set)
	}
}
