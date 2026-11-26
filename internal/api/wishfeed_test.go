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

// A kid who looks up a book above their limit sees nothing about it, can ask for access, and only then does a parent hear of it.
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
	if _, n := admin.do("GET", "/api/notifications", nil, false); strings.Contains(toJSON(n), "Too Spicy") {
		t.Fatalf("nobody is told just because a kid looked: %v", n)
	}
	ask := func() int {
		req, _ := http.NewRequest("POST", srv.URL+"/kosync/plugin/access-request", strings.NewReader(`{"book_id":`+itoa(id)+`}`))
		req.Header.Set("x-auth-user", "kid")
		req.Header.Set("x-auth-key", key)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if code := ask(); code != 200 {
		t.Fatalf("ask for access: %d", code)
	}
	if code := ask(); code != 409 {
		t.Fatalf("asked twice: %d", code)
	}
	if _, n := admin.do("GET", "/api/notifications", nil, false); !strings.Contains(toJSON(n), "asked for access to") {
		t.Fatalf("the parent sees the ask: %v", n)
	}
	if _, q := admin.do("GET", "/api/requests", nil, false); len(q["open"].([]any)) != 1 {
		t.Fatalf("the ask is a request a parent can answer: %v", q)
	}
}

// A grown-up with a spice limit still sees a book's rating when she checks it, but the library is not shown to her: no download,
// no Nightstand, no request.
func TestRatingOnlyAboveLimit(t *testing.T) {
	srv, st := setup(t)
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]any{"username": "wife", "password": "wifepass12", "role": "editor"}, true)
	_, _ = st.DB.Exec(`UPDATE users SET max_spice = 3 WHERE username = 'wife'`)
	wife := login(t, srv, "wife", "wifepass12")
	_, ks := wife.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	key := hex.EncodeToString(sum[:])
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	four := 4
	id, _ := st.UpsertBook("Over Three", "Someone", "", "")
	_ = st.AddCopy(cal, id, "/c/o.epub", "epub", "1")
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &four, Model: "gpt"})
	req, _ := http.NewRequest("GET", srv.URL+"/kosync/plugin/info/"+itoa(id), nil)
	req.Header.Set("x-auth-user", "wife")
	req.Header.Set("x-auth-key", key)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(res.Body).Decode(&out)
	if res.StatusCode != 200 || out["rating_only"] != true || out["level"] != float64(4) || out["url"] != nil || out["in_library"] != nil || out["request"] != nil {
		t.Fatalf("rating only: %d %v", res.StatusCode, out)
	}
}

// A wish is also met when the library keeps the same book under another spelling (a second record with the same title words and
// author surname), in the format asked for.
func TestWishMetByAnotherRecordOfTheBook(t *testing.T) {
	_, st := setup(t)
	lookedUp, _ := st.EnsureCatalog(store.LookedUpCatalog, "custom")
	abs, _ := st.EnsureCatalog(store.AudiobookshelfCatalog, "custom")
	wish, _ := st.UpsertBook("Cress", "Marissa Meyer", "", "")
	_ = st.AddCopy(lookedUp, wish, "lookup:Cress", "list", "")
	lib, _ := st.UpsertBook("The Cress", "Meyer, Marissa", "", "")
	if lib == wish {
		t.Skip("the library merged them already")
	}
	u, _ := st.UserByName("admin")
	if err := st.AddWishFormat(wish, u, "", "audio"); err != nil {
		t.Fatal(err)
	}
	var told int
	st.OnWishAcquired = func(store.Wish) { told++ }
	st.MarkAcquired()
	if told != 0 {
		t.Fatal("nothing has arrived yet")
	}
	_ = st.AddCopy(abs, lib, "abs:1", "audio", "1")
	st.MarkAcquired()
	st.MarkAcquired()
	if told != 1 {
		t.Fatalf("told %d times", told)
	}
}
