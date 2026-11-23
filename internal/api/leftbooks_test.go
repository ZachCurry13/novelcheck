package api_test

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// A book that leaves the e-reader is asked about once: keep it, remove it from my library (hidden for this person
// only, shown again from Settings) or ask to delete it; a store book removed to the cloud counts as leaving.
func TestBooksLeavingTheKindle(t *testing.T) {
	srv, st := setup(t)
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]any{"username": "wife", "password": "wifepass12", "role": "editor"}, true)
	wife := login(t, srv, "wife", "wifepass12")
	_, ks := wife.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	report := func(books ...map[string]any) []any {
		raw, _ := json.Marshal(map[string]any{"device": "Kindle (Amazon reader)", "plugin_version": "app 1.2.0", "books": books})
		req, _ := http.NewRequest("POST", srv.URL+"/kosync/plugin/device", bytes.NewReader(raw))
		req.Header.Set("x-auth-user", "wife")
		req.Header.Set("x-auth-key", hex.EncodeToString(sum[:]))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		out := map[string]any{}
		_ = json.Unmarshal(b, &out)
		left, _ := out["left"].([]any)
		return left
	}
	dirty, _ := st.UpsertBook("Dirty Book", "A. Writer", "", "")
	clean, _ := st.UpsertBook("Clean Book", "B. Writer", "", "")
	cal, _ := st.EnsureCatalog("Family", "custom")
	_ = st.AddCopy(cal, dirty, "x", "list", "")
	_ = st.AddCopy(cal, clean, "y", "list", "")
	d := map[string]any{"filename": "dirty.azw3", "title": "Dirty Book", "author": "A. Writer", "format": "azw3"}
	c := map[string]any{"filename": "clean.azw3", "title": "Clean Book", "author": "B. Writer", "format": "azw3"}
	if left := report(d, c); len(left) != 0 {
		t.Fatalf("a first report leaves nothing: %v", left)
	}
	if left := report(c); len(left) != 1 || left[0].(map[string]any)["title"] != "Dirty Book" {
		t.Fatalf("the deleted book is asked about: %v", left)
	}
	if left := report(c); len(left) != 0 {
		t.Fatalf("asked once only: %v", left)
	}
	_, mine := wife.do("GET", "/api/me/left", nil, false)
	if l, _ := mine["left"].([]any); len(l) != 1 {
		t.Fatalf("it waits in Settings: %v", mine)
	}
	if res, _ := wife.do("POST", "/api/me/left", map[string]any{"book_id": dirty, "choice": "hide"}, true); res.StatusCode != 200 {
		t.Fatalf("hide: %d", res.StatusCode)
	}
	if res, _ := wife.do("GET", "/api/books/"+itoa(dirty), nil, false); res.StatusCode != 404 {
		t.Fatalf("hidden from her: %d", res.StatusCode)
	}
	if res, _ := admin.do("GET", "/api/books/"+itoa(dirty), nil, false); res.StatusCode != 200 {
		t.Fatalf("still there for everyone else: %d", res.StatusCode)
	}
	if b, _ := st.BookByID(dirty, nil); b == nil {
		t.Fatal("the book and its rating stay")
	}
	wife.do("DELETE", "/api/me/hidden-books/"+itoa(dirty), nil, true)
	if res, _ := wife.do("GET", "/api/books/"+itoa(dirty), nil, false); res.StatusCode != 200 {
		t.Fatalf("Show again: %d", res.StatusCode)
	}
	cloud := map[string]any{"filename": "Clean Book [B0X]", "title": "Clean Book", "author": "B. Writer", "format": "cloud"}
	if left := report(cloud); len(left) != 1 {
		t.Fatalf("removed to the Amazon cloud counts as leaving: %v", left)
	}
	if res, _ := wife.do("POST", "/api/me/left", map[string]any{"book_id": clean, "choice": "delete"}, true); res.StatusCode != 200 {
		t.Fatalf("ask to delete: %d", res.StatusCode)
	}
	if res, _ := wife.do("GET", "/api/books/"+itoa(clean), nil, false); res.StatusCode != 404 {
		t.Fatalf("asked to delete: hidden from her at once: %d", res.StatusCode)
	}
}
