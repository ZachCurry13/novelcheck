package api_test

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The wish feed is public behind its secret, shows title, author and format but never who asked, and a book leaves it (and the
// person who asked is told, once) when it arrives in a library.
func TestWishFeedAndArrival(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	lookedUp, _ := st.EnsureCatalog(store.LookedUpCatalog, "custom")
	calibre, _ := st.EnsureCatalog("Calibre Main", "calibre")
	id, _ := st.UpsertBook("Fourth Wing", "Rebecca Yarros", "", "")
	_ = st.AddCopy(lookedUp, id, "lookup:Fourth Wing", "list", "")
	var told []store.Wish
	st.OnWishAcquired = func(w store.Wish) { told = append(told, w) }

	if res, out := admin.do("POST", "/api/books/"+itoa(id)+"/wish", map[string]string{"format": "ebook"}, true); res.StatusCode != 200 {
		t.Fatalf("wish: %d %v", res.StatusCode, out)
	}
	feed := func(path string) (int, string) {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	if code, _ := feed("/feed/wishes/nothing"); code != 404 {
		t.Fatalf("feed off: %d", code)
	}
	_, on := admin.do("POST", "/api/admin/wish-feed", map[string]string{}, true)
	url, _ := on["url"].(string)
	if url == "" {
		t.Fatalf("turn on: %v", on)
	}
	path := url[strings.Index(url, "/feed/"):]
	code, body := feed(path)
	if code != 200 || !strings.Contains(body, "Fourth Wing by Rebecca Yarros (ebook)") || strings.Contains(body, "admin") {
		t.Fatalf("feed: %d %s", code, body)
	}
	if code, _ := feed(path + "x"); code != 404 {
		t.Fatalf("wrong secret: %d", code)
	}

	// the e-book arrives: it leaves the feed, the person is told once
	_ = st.AddCopy(calibre, id, "/c/fw.epub", "epub", "1")
	st.MarkAcquired()
	st.MarkAcquired()
	if len(told) != 1 || told[0].Username != "admin" || told[0].Title != "Fourth Wing" {
		t.Fatalf("told: %v", told)
	}
	if _, body := feed(path); strings.Contains(body, "Fourth Wing") {
		t.Fatalf("it should have left the feed: %s", body)
	}
	if res, _ := admin.do("DELETE", "/api/admin/wish-feed", nil, true); res.StatusCode != 200 {
		t.Fatalf("off: %d", res.StatusCode)
	}
	if code, _ := feed(path); code != 404 {
		t.Fatalf("after off: %d", code)
	}
}

// A kid who looks up a book above their limit gets a plain "not available" and nothing about the book; the parents are told.
func TestLookupBlockedForKid(t *testing.T) {
	srv, st := setup(t)
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12", "role": "restricted"}, true)
	_, _ = st.DB.Exec(`UPDATE users SET max_spice = 1, ai_features = 1 WHERE username = 'kid'`)
	kid := login(t, srv, "kid", "kidpass12")
	_, ks := kid.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	key := hex.EncodeToString(sum[:])
	four := 4
	id, _ := st.UpsertBook("Too Spicy", "Someone", "", "")
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &four, Model: "gpt"})
	req, _ := http.NewRequest("GET", srv.URL+"/kosync/plugin/info/"+itoa(id), nil)
	req.Header.Set("x-auth-user", "kid")
	req.Header.Set("x-auth-key", key)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(res.Body).Decode(&out)
	if res.StatusCode != 200 || out["blocked"] != true || out["title"] != nil || out["level"] != nil {
		t.Fatalf("blocked info: %d %v", res.StatusCode, out)
	}
	if _, n := admin.do("GET", "/api/notifications", nil, false); !strings.Contains(toJSON(n), "outside their rules") {
		t.Fatalf("parents told: %v", n)
	}
}
