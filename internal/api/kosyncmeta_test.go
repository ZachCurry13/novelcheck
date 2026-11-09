package api_test

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// KOReader 2026.07 can send the book's title and author with progress. A book NovelCheck never sent
// to the reader is then found by them: it moves along the Nightstand, and its fingerprint is remembered.
func TestKosyncProgressWithMetadata(t *testing.T) {
	srv, st := setup(t)
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, c := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(c["code"].(string)))
	key := hex.EncodeToString(sum[:])
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	u, _ := st.UserByName("admin")
	book, _ := st.UpsertBook("The Search for Sam", "Pittacus Lore", "", "")
	_ = st.AddCopy(cal, book, "/c/sam.epub", "epub", "5")
	_ = st.Enqueue(u.ID, book)
	put := func(body string) int {
		req, _ := http.NewRequest("PUT", srv.URL+"/kosync/syncs/progress", bytes.NewBufferString(body))
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", key)
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	status := func() string {
		var s string
		_ = st.DB.Get(&s, `SELECT status FROM queue_items WHERE user_id = ? AND book_id = ?`, u.ID, book)
		return s
	}
	// Without the metadata nothing links this fingerprint to the book.
	if put(`{"document":"abc123","progress":"10","percentage":0.2,"device":"Kindle","device_id":"k1"}`) != 200 || status() != "queued" {
		t.Fatalf("unknown fingerprint, no metadata: %q", status())
	}
	// With it, the book is found by title and author.
	body := `{"document":"abc123","metadata":{"filename":"sam.epub","title":"The Search for Sam","authors":"Pittacus Lore"},"progress":"12","percentage":0.3,"device":"Kindle","device_id":"k1"}`
	if put(body) != 200 || status() != "reading" {
		t.Fatalf("found by metadata: %q", status())
	}
	if st.DocBook("abc123") != book {
		t.Fatal("the fingerprint is remembered")
	}
	// A title that matches nothing in the library does nothing.
	if put(`{"document":"zzz9","metadata":{"title":"Some Other Book","authors":"Nobody"},"progress":"1","percentage":0.5,"device":"K","device_id":"k"}`) != 200 {
		t.Fatal("put")
	}
	if st.DocBook("zzz9") != 0 {
		t.Fatal("no match, no link")
	}
}
