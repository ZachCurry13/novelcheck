package api_test

import (
	"net/http"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The cleanup lists are kept: the second visit is answered from memory, and a change to the library throws
// the kept answer away.
func TestCleanupListsAreKept(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cacheOf := func() string {
		req, _ := http.NewRequest("GET", admin.base+"/api/admin/same-books", nil)
		res, err := admin.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.Header.Get("X-Cache")
	}
	if got := cacheOf(); got != "miss" {
		t.Fatalf("first visit: %q", got)
	}
	if got := cacheOf(); got != "hit" {
		t.Fatalf("second visit: %q", got)
	}
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	id, _ := st.UpsertBook("Cinder", "Marissa Meyer", "", "")
	_ = st.AddCopy(cal, id, "/c/c.epub", "epub", "9") // a sync adding a book, with no request involved
	if got := cacheOf(); got != "miss" {
		t.Fatalf("after the library changed: %q", got)
	}
}
