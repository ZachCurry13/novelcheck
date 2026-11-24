package api_test

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The Kindle app's Change cover: only cover addresses, only books you can see, and "back to the usual one" works.
func TestKindleAppChangeCover(t *testing.T) {
	srv, st := setup(t)
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, ks := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	id, _ := st.UpsertBook("Divergent", "Veronica Roth", "", "")
	post := func(body string) int {
		req, _ := http.NewRequest("POST", srv.URL+"/kosync/plugin/cover", strings.NewReader(body))
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", hex.EncodeToString(sum[:]))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	bid := itoa(id)
	if got := post(`{"book_id":` + bid + `,"url":"https://evil.example/c.jpg"}`); got != 400 {
		t.Fatalf("a cover from anywhere: %d", got)
	}
	if got := post(`{"book_id":99999,"url":"https://covers.openlibrary.org/b/id/1-L.jpg"}`); got != 404 {
		t.Fatalf("a book that isn't there: %d", got)
	}
	if got := post(`{"book_id":` + bid + `,"reset":true}`); got != 200 {
		t.Fatalf("back to the usual cover: %d", got)
	}
}

// The Kindle app can take a book off the Nightstand; it stays off (hidden, so the e-reader doesn't add it back) until it is added again.
func TestKindleAppRemovesFromNightstand(t *testing.T) {
	srv, st := setup(t)
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	_ = st.SetSetting(store.KeyModuleQueue, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, ks := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	u, _ := st.UserByName("admin")
	id, _ := st.UpsertBook("Alice's Adventures in Wonderland", "Lewis Carroll", "", "")
	post := func(path string) int {
		req, _ := http.NewRequest("POST", srv.URL+"/kosync/plugin/nightstand"+path, strings.NewReader(`{"book_id":`+itoa(id)+`}`))
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", hex.EncodeToString(sum[:]))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if got := post("/remove"); got != 404 {
		t.Fatalf("not on the Nightstand yet: %d", got)
	}
	_ = st.Enqueue(u.ID, id)
	if got := post("/remove"); got != 200 {
		t.Fatalf("remove: %d", got)
	}
	if items, _ := st.ListQueue(u.ID, false); len(items) != 0 {
		t.Fatalf("still on the Nightstand: %v", items)
	}
	if st.EnqueueAuto(u.ID, id, "queued") {
		t.Fatal("the e-reader put it back")
	}
	if got := post(""); got != 200 {
		t.Fatalf("add again: %d", got)
	}
	if items, _ := st.ListQueue(u.ID, false); len(items) != 1 {
		t.Fatalf("not back: %v", items)
	}
}
