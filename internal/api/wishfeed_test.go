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

// Asking the AI about a book: a kid needs the parents' AI switch, a book that is not rated yet cannot be asked about.
func TestAskBookRules(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12", "role": "restricted"}, true)
	_, _ = st.DB.Exec(`UPDATE users SET max_spice = -1, hide_unrated = 0 WHERE username = 'kid'`)
	kid := login(t, srv, "kid", "kidpass12")
	id, _ := st.UpsertBook("Unrated", "Someone", "", "")
	if res, _ := kid.do("POST", "/api/books/"+itoa(id)+"/ask", map[string]string{"question": "Who?"}, true); res.StatusCode != 403 {
		t.Fatalf("a kid without the AI switch: %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", "/api/books/"+itoa(id)+"/ask", map[string]string{"question": "Who?"}, true); res.StatusCode != 409 {
		t.Fatalf("a book not rated yet: %d", res.StatusCode)
	}
}

// The bug log lists what was removed from Calibre and which Kindles still have a file with that title.
func TestBugLogShowsRemovalsAndKindles(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	_, _ = st.DB.Exec(`INSERT INTO calibre_removals (by_user, tool, calibre_id, title) VALUES ('admin', 'duplicates', 77, 'Brave New World')`)
	u, _ := st.UserByName("admin")
	_, _ = st.DB.Exec(`INSERT INTO device_books (user_id, device, filename, title, reported_at) VALUES (?, 'Kindle', 'Brave New World - Aldous Huxley (151).epub', '', 1)`, u.ID)
	_, out := admin.do("POST", "/api/admin/bug-link", map[string]string{}, true)
	url, _ := out["url"].(string)
	res, err := http.Get(srv.URL + url[strings.Index(url, "/bugs/"):])
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(b), "Calibre #77 · Brave New World · on a Kindle now: admin / Kindle") {
		t.Fatalf("log: %s", string(b)[:min(len(b), 600)])
	}
}

// A person's default libraries are saved for them alone, and a library that is gone is left out.
func TestLibraryDefault(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	if res, out := admin.do("PUT", "/api/me/library-default", map[string]any{"catalogs": []int64{cal, 9999}}, true); res.StatusCode != 200 || len(out["catalogs"].([]any)) != 1 {
		t.Fatalf("save: %d %v", res.StatusCode, out)
	}
	admin.do("POST", "/api/admin/users", map[string]any{"username": "other", "password": "otherpass12"}, true)
	other := login(t, srv, "other", "otherpass12")
	if _, out := other.do("GET", "/api/me/library-default", nil, false); len(out["catalogs"].([]any)) != 0 {
		t.Fatalf("someone else's default: %v", out)
	}
	if _, out := admin.do("PUT", "/api/me/library-default", map[string]any{"catalogs": []int64{}}, true); len(out["catalogs"].([]any)) != 0 {
		t.Fatalf("clear: %v", out)
	}
}

// Digital shows books with an e-book or audiobook copy, Physical the books on the shelf (a paper copy).
func TestDigitalAndPhysicalLibrary(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	shelf, _ := st.EnsureCatalog("Living Room Shelf", "custom")
	digital, _ := st.UpsertBook("On My Kindle", "Author One", "", "")
	_ = st.AddCopy(cal, digital, "/c/a.epub", "epub", "1")
	paper, _ := st.UpsertBook("On The Shelf", "Author Two", "", "")
	_ = st.AddCopy(shelf, paper, "paper", "paper", "")
	both, _ := st.UpsertBook("Both Ways", "Author Three", "", "")
	_ = st.AddCopy(cal, both, "/c/b.epub", "epub", "2")
	_ = st.AddCopy(shelf, both, "paper-b", "paper", "")
	titles := func(q string) string {
		_, out := admin.do("GET", "/api/books?"+q, nil, false)
		var names []string
		for _, b := range out["books"].([]any) {
			names = append(names, b.(map[string]any)["title"].(string))
		}
		return strings.Join(names, "|")
	}
	_, all := admin.do("GET", "/api/books?sort=title", nil, false)
	for _, b := range all["books"].([]any) { // the cards say where a book is from these two fields
		m := b.(map[string]any)
		if m["title"] == "Both Ways" && (!strings.Contains(m["formats"].(string), "EPUB") || !strings.Contains(m["formats"].(string), "PAPER") || !strings.Contains(m["catalogs"].(string), "Living Room Shelf")) {
			t.Fatalf("card data: %v", m)
		}
	}
	if got := titles("medium=digital&sort=title"); got != "Both Ways|On My Kindle" {
		t.Fatalf("digital: %q", got)
	}
	if got := titles("medium=physical&sort=title"); got != "Both Ways|On The Shelf" {
		t.Fatalf("physical: %q", got)
	}
	if got := titles("sort=title"); got != "Both Ways|On My Kindle|On The Shelf" {
		t.Fatalf("all: %q", got)
	}
}

// A person's two Kindle libraries are joined into one, keeping every book once, and the Kindle app's store books then go there.
func TestJoinKindleLibraries(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	u, _ := st.UserByName("admin")
	amazon, _ := st.CreateOwnedCatalog("Zach's Amazon Kindle library", "drive", u.ID, true)
	store1, _ := st.CreateOwnedCatalog("admin's Kindle store", "drive", u.ID, true)
	a, _ := st.UpsertBook("Book A", "Au", "", "")
	b, _ := st.UpsertBook("Book B", "Au", "", "")
	_ = st.AddCopy(amazon, a, "amazon:a", "kindle", "a")
	_ = st.AddCopy(amazon, b, "amazon:b", "kindle", "b")
	_ = st.AddCopy(store1, b, "kindle:b", "kindle", "b")
	_, out := admin.do("GET", "/api/admin/catalogs/kindle-pairs", nil, false)
	pairs := out["pairs"].([]any)
	if len(pairs) != 1 || pairs[0].(map[string]any)["into"] != float64(amazon) || pairs[0].(map[string]any)["new_name"] != "admin's Kindle" {
		t.Fatalf("pairs: %v", out)
	}
	p := pairs[0].(map[string]any)
	if res, o := admin.do("POST", "/api/admin/catalogs/merge", map[string]any{"from": p["from"], "into": p["into"], "name": p["new_name"]}, true); res.StatusCode != 200 || len(o["pairs"].([]any)) != 0 {
		t.Fatalf("merge: %d %v", res.StatusCode, o)
	}
	var n int
	_ = st.DB.Get(&n, `SELECT COUNT(DISTINCT book_id) FROM catalog_books WHERE catalog_id = ?`, amazon)
	c, err := st.CatalogByID(amazon)
	if err != nil || c.Name != "admin's Kindle" || n != 2 {
		t.Fatalf("joined library: %v %v books=%d", c, err, n)
	}
	if _, err := st.CatalogByID(store1); err == nil {
		t.Fatal("the smaller library should be gone")
	}
	if id, _ := st.StoreCatalog(u); id != amazon {
		t.Fatalf("the Kindle app's store books go to the joined library, got %d", id)
	}
	if res, _ := admin.do("POST", "/api/admin/catalogs/merge", map[string]any{"from": amazon, "into": amazon, "name": "x"}, true); res.StatusCode != 400 {
		t.Fatalf("same library: %d", res.StatusCode)
	}
}

// The Not sure list shows every unsure rating, the nightly number is kept, and a batch needs the Deep Scan machine.
func TestNotSureList(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	two := 2
	id, _ := st.UpsertBook("Unsure Book", "An Author", "", "")
	_ = st.AddCopy(cal, id, "/c/u.epub", "epub", "1")
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &two, Model: "small", Confidence: "low"})
	_, out := admin.do("GET", "/api/admin/not-sure", nil, false)
	books, _ := out["books"].([]any)
	if len(books) != 1 || out["waiting"] != float64(1) || out["machine"] != false {
		t.Fatalf("list: %v", out)
	}
	if _, o := admin.do("PUT", "/api/admin/not-sure", map[string]int{"nightly": 10}, true); o["nightly"] != float64(10) {
		t.Fatalf("nightly: %v", o)
	}
	if res, _ := admin.do("PUT", "/api/admin/not-sure", map[string]int{"nightly": -1}, true); res.StatusCode != 400 {
		t.Fatalf("a negative number: %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", "/api/admin/not-sure/run", map[string]int{"limit": 0}, true); res.StatusCode != 409 {
		t.Fatalf("no Deep Scan machine: %d", res.StatusCode)
	}
}

// An identifier Amazon puts after an author is taken out, for new reports and for books already made with it; a real name stays.
func TestJunkAuthorIdentifier(t *testing.T) {
	if got := store.StripJunkToken("F. C. Yee FOLF7SLTZ3Y67TE2QPL6EVXGMLIPU6Z"); got != "F. C. Yee" {
		t.Fatalf("stripped: %q", got)
	}
	for _, name := range []string{"Brandon Sanderson", "J. R. R. TOLKIEN", "ABCDEFGHIJKLMNOPQRSTUVWXYZ"} { // no digits: not an identifier
		if store.StripJunkToken(name) != name {
			t.Fatalf("%q changed", name)
		}
	}
	_, st := setup(t)
	junk, _ := st.UpsertBook("Shadow of Kyoshi", "F. C. Yee FOLF7SLTZ3Y67TE2QPL6EVXGMLIPU6Z", "", "")
	if fixed := st.RepairJunkAuthors(); len(fixed) != 1 || fixed[0] != junk {
		t.Fatalf("repaired: %v", fixed)
	}
	if b, _ := st.BookByID(junk, nil); b.Author != "F. C. Yee" {
		t.Fatalf("author: %q", b.Author)
	}
	if id := st.MatchBook("Shadow of Kyoshi", "F. C. Yee"); id != junk {
		t.Fatalf("found by its real author: %d", id)
	}
}

// A Kindle on an older app gets one notice per new version, the Health line says how far behind it is, and an admin can remind.
func TestUpdateNudge(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	u, _ := st.UserByName("admin")
	st.SaveDeviceInfo(u.ID, "Old Kindle", "app 1.2.0")
	_, out := admin.do("GET", "/api/admin/health", nil, false)
	found := false
	for _, it := range out["items"].([]any) {
		m := it.(map[string]any)
		if strings.Contains(m["title"].(string), "Old Kindle") && strings.Contains(m["detail"].(string), "versions behind") && m["remind"] != nil {
			found = true
		}
	}
	if !found {
		t.Fatalf("health line: %v", out["items"])
	}
	if res, o := admin.do("POST", "/api/admin/devices/remind", map[string]any{"user_id": u.ID, "device": "Old Kindle"}, true); res.StatusCode != 200 || o["sent"] != false {
		t.Fatalf("remind (no phone notifications on): %d %v", res.StatusCode, o)
	}
	if res, _ := admin.do("POST", "/api/admin/devices/remind", map[string]any{"user_id": u.ID, "device": "Nope"}, true); res.StatusCode != 404 {
		t.Fatalf("unknown Kindle: %d", res.StatusCode)
	}
	st.SaveDeviceInfo(u.ID, "Old Kindle", "app 1.2.1")
	if _, o := admin.do("GET", "/api/admin/health", nil, false); o == nil {
		t.Fatal("health")
	}
}

// After a restart the books that were being rated, then those waiting, are listed to be queued again (in that order).
func TestInterruptedRatingsComeBack(t *testing.T) {
	_, st := setup(t)
	a, _ := st.UpsertBook("Waiting", "A", "", "")
	b, _ := st.UpsertBook("Being rated", "B", "", "")
	c, _ := st.UpsertBook("Pending", "C", "", "")
	_ = st.SetStatus(a, "queued", "")
	_ = st.SetStatus(b, "processing", "")
	ids := st.InterruptedRatings()
	if len(ids) != 2 || ids[0] != b || ids[1] != a {
		t.Fatalf("interrupted: %v (c=%d)", ids, c)
	}
}

// The pile of "main AI isn't working" notices an earlier version made is cleared; a different notice is left alone.
func TestClearRepeatedBackupNotes(t *testing.T) {
	_, st := setup(t)
	for i := 1; i <= 3; i++ {
		st.Notify("warning", "llm-backup", "The main AI isn't working. Its last answer — gemini: used up; NovelCheck will ask it again in "+strings.Repeat("1", i)+" h", "#/system")
	}
	st.Notify("warning", "llm-backup", "Some other backup notice", "#/system")
	if n := st.ClearRepeatedBackupNotes(); n != 3 {
		t.Fatalf("cleared %d", n)
	}
	_, unread, _ := st.Notifications(10)
	if unread != 1 {
		t.Fatalf("unread: %d", unread)
	}
}
