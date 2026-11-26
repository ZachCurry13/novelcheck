package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Home shows a person their own things; a parent also gets the asks waiting for them; an admin the inbox and health.
func TestHomeForEachKindOfPerson(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	_, ah := admin.do("GET", "/api/home", nil, false)
	if ah["inbox"] == nil || ah["health"] == nil || ah["requests"] == nil {
		t.Fatalf("an admin gets the inbox, health and asks: %v", ah)
	}
	_, kh := kid.do("GET", "/api/home", nil, false)
	if kh["inbox"] != nil || kh["health"] != nil || kh["requests"] != nil || kh["next_up"] == nil {
		t.Fatalf("a kid gets only their own things: %v", kh)
	}
	if res, _ := kid.do("GET", "/api/admin/health", nil, false); res.StatusCode != http.StatusForbidden {
		t.Fatalf("health is for admins: %d", res.StatusCode)
	}
	_ = st
}

// A kid asks for a book; a parent says yes: the book is OK for kids and on the kid's Nightstand. A book not in the library is approved and waits.
func TestRequestsAndApprovals(t *testing.T) {
	srv, st := setup(t)
	_ = st.SetSetting(store.KeyModuleQueue, "true")
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	have, _ := st.UpsertBook("Divergent", "Veronica Roth", "", "")
	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	_ = st.AddCopy(cat, have, "/c/divergent.epub", "epub", "1")

	if res, out := kid.do("POST", "/api/requests", map[string]any{"title": "Divergent", "author": "Veronica Roth"}, true); res.StatusCode != 200 {
		t.Fatalf("ask: %d %v", res.StatusCode, out)
	}
	if res, _ := kid.do("POST", "/api/requests", map[string]any{"title": "Divergent"}, true); res.StatusCode != 400 {
		t.Fatalf("the same ask twice: %d", res.StatusCode)
	}
	kid.do("POST", "/api/requests", map[string]any{"title": "A Book Nobody Has", "author": "Nobody"}, true)
	_, open := admin.do("GET", "/api/requests", nil, false)
	list, _ := open["open"].([]any)
	if len(list) != 2 {
		t.Fatalf("a parent sees both asks: %v", open["open"])
	}
	if res, _ := kid.do("POST", "/api/requests/"+itoa(int64(list[0].(map[string]any)["id"].(float64)))+"/decide", map[string]any{"action": "approve"}, true); res.StatusCode != 403 {
		t.Fatalf("a kid can't answer: %d", res.StatusCode)
	}
	first, second := int64(list[0].(map[string]any)["id"].(float64)), int64(list[1].(map[string]any)["id"].(float64))
	if res, out := admin.do("POST", "/api/requests/"+itoa(first)+"/decide", map[string]any{"action": "approve"}, true); res.StatusCode != 200 {
		t.Fatalf("approve: %d %v", res.StatusCode, out)
	}
	kidUser, _ := st.UserByName("kid")
	if items, _ := st.ListQueue(kidUser.ID, false); len(items) != 1 {
		t.Fatalf("the approved book should be on the kid's Nightstand: %v", items)
	}
	if b, _ := st.BookByID(have, nil); !b.Approved {
		t.Fatal("the book should be marked OK for kids")
	}
	admin.do("POST", "/api/requests/"+itoa(second)+"/decide", map[string]any{"action": "approve"}, true)
	_, open = admin.do("GET", "/api/requests", nil, false)
	if list, _ = open["open"].([]any); len(list) != 1 || list[0].(map[string]any)["status"] != "approved" {
		t.Fatalf("the book to get stays in the inbox: %v", open["open"])
	}
}

// "I disagree" is kept with what the AI said, and the Accuracy page scores it.
func TestDisagreeAndAccuracy(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	id, _ := st.UpsertBook("Fourth Wing", "Rebecca Yarros", "", "")
	lvl := 3
	_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &lvl, Model: "test-model"})
	if res, _ := kid.do("POST", "/api/books/"+itoa(id)+"/disagree", map[string]any{"level": 4}, true); res.StatusCode != 403 {
		t.Fatalf("a kid doesn't give levels: %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", "/api/books/"+itoa(id)+"/disagree", map[string]any{"level": 9}, true); res.StatusCode != 400 {
		t.Fatalf("level 9: %d", res.StatusCode)
	}
	if res, out := admin.do("POST", "/api/books/"+itoa(id)+"/disagree", map[string]any{"level": 4, "note": "it is open door"}, true); res.StatusCode != 200 {
		t.Fatalf("disagree: %d %v", res.StatusCode, out)
	}
	_, acc := admin.do("GET", "/api/admin/accuracy", nil, false)
	models, _ := acc["models"].([]any)
	if len(models) < 2 {
		t.Fatalf("models: %v", acc)
	}
	all := models[0].(map[string]any)
	if all["model"] != "All" || all["n"] != float64(1) || all["exact"] != float64(0) || all["within1"] != float64(1) || all["lower"] != float64(1) {
		t.Fatalf("the AI said 3, the person 4: %v", all)
	}
	recent, _ := acc["recent"].([]any)
	fid := int64(recent[0].(map[string]any)["id"].(float64))
	if res, _ := admin.do("POST", "/api/admin/accuracy/"+itoa(fid)+"/use", nil, true); res.StatusCode != 200 {
		t.Fatalf("use: %d", res.StatusCode)
	}
	if b, _ := st.BookByID(id, nil); b.SpiceLevel == nil || *b.SpiceLevel != 4 {
		t.Fatalf("the book should have the person's level: %v", b.SpiceLevel)
	}
}

// "Don't convert" on the Convert to EPUB list is remembered: the book is not offered again.
func TestDontConvertIsRemembered(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	id, _ := st.UpsertBook("Notes", "Somebody", "", "")
	_ = st.AddCopy(cat, id, "/c/notes.mobi", "mobi", "77")
	count := func() int {
		_, out := admin.do("GET", "/api/admin/formats", nil, false)
		n, _ := out["convert_count"].(float64)
		return int(n)
	}
	if count() != 1 {
		t.Fatalf("the book should be offered for converting: %d", count())
	}
	if res, out := admin.do("POST", "/api/admin/formats/convert/as-is", map[string]any{"calibre_id": "77", "as_is": true, "title": "Notes", "format": "mobi"}, true); res.StatusCode != 200 {
		t.Fatalf("don't convert: %d %v", res.StatusCode, out)
	}
	if count() != 0 {
		t.Fatalf("it should not be offered again: %d", count())
	}
}

// The backups on the Health page: Back up now makes a copy, everyone who is an admin sees the list, only the main admin downloads.
func TestBackupsPage(t *testing.T) {
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.DataDir = t.TempDir() })
	_ = st
	admin := login(t, srv, "admin", "adminpass1")
	_, none := admin.do("GET", "/api/admin/backups", nil, false)
	if files, _ := none["files"].([]any); len(files) != 0 {
		t.Fatalf("no copy yet: %v", none)
	}
	_, made := admin.do("POST", "/api/admin/backups/now", nil, true)
	files, _ := made["files"].([]any)
	if len(files) != 1 {
		t.Fatalf("one copy after Back up now: %v", made)
	}
	name := files[0].(map[string]any)["name"].(string)
	req, _ := http.NewRequest("GET", srv.URL+"/api/admin/backups/"+name, nil)
	res, err := admin.http.Do(req)
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("the main admin downloads it: %v %v", err, res)
	}
	res.Body.Close()
	if res, _ := admin.do("GET", "/api/admin/backups/not-a-backup.db", nil, false); res.StatusCode != 404 {
		t.Fatalf("only backup names: %d", res.StatusCode)
	}
}

// Health names a Calibre Content server that is behind and an error that keeps repeating.
func TestHealthSaysWhatIsRepeating(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	st.Notify("warning", "calibre-behind", "Calibre's Content server lacks 3 books", "")
	for i := 0; i < 6; i++ {
		_, _ = st.DB.Exec(`INSERT INTO error_log (code, at, method, path, status, username, detail) VALUES ('BG-x', ?, 'background', 'deep scan', 0, '', ?)`,
			time.Now().UTC().Format("2006-01-02 15:04:05"), fmt.Sprintf("llama-server crashed (pid %d)", 100+i))
	}
	_, h := admin.do("GET", "/api/admin/health", nil, false)
	text, _ := json.Marshal(h["items"])
	for _, want := range []string{"Calibre's Content server is behind", "The same error 6 times"} {
		if !strings.Contains(string(text), want) {
			t.Fatalf("health lacks %q: %s", want, text)
		}
	}
}

// Tidy backups removes Amazon's dictionaries and the Kindle app's files that the plugin's backup of the same Kindle holds
// identically, and nothing else (a file with the same name but other content stays).
func TestTidyBackups(t *testing.T) {
	data := t.TempDir()
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.DataDir = data })
	admin := login(t, srv, "admin", "adminpass1")
	u, _ := st.UserByName("admin")
	const appName, pluginName = "Zach Paperwhite (Amazon reader)", "Zach Paperwhite"
	for _, d := range []string{appName, pluginName} {
		_, _ = st.SaveDeviceBooks(u.ID, d, []store.DeviceBook{{Filename: "x.epub", Title: "X"}})
	}
	_ = st.LinkDevice(u.ID, appName, pluginName)
	put := func(device, rel, body string) {
		dir := filepath.Join(data, "kindle-backup", itoa(u.ID), device)
		_ = os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0o755)
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put(appName, "documents/Same Book.epub", "the same bytes")
	put(pluginName, "documents/Books/Same Book.epub", "the same bytes")
	put(appName, "documents/Other.epub", "app version!!") // same name and size as the plugin's, other content
	put(pluginName, "documents/Other.epub", "plugin version")
	put(appName, "documents/dictionaries/Oxford.azw", "a dictionary")
	_, plan := admin.do("GET", "/api/admin/backups/tidy", nil, false)
	if plan["count"] != float64(2) {
		t.Fatalf("the duplicate and the dictionary: %v", plan)
	}
	_, done := admin.do("POST", "/api/admin/backups/tidy", nil, true)
	if done["removed"] != float64(2) {
		t.Fatalf("removed: %v", done)
	}
	for rel, want := range map[string]bool{"documents/Same Book.epub": false, "documents/dictionaries/Oxford.azw": false, "documents/Other.epub": true} {
		_, err := os.Stat(filepath.Join(data, "kindle-backup", itoa(u.ID), appName, filepath.FromSlash(rel)))
		if (err == nil) != want {
			t.Fatalf("%s kept = %v, want %v", rel, err == nil, want)
		}
	}
	if _, err := os.Stat(filepath.Join(data, "kindle-backup", itoa(u.ID), pluginName, "documents/Books/Same Book.epub")); err != nil {
		t.Fatal("the plugin's copy must stay")
	}
}

// Health says everything about one Kindle on one line (not a line per problem), and a Kindle that stopped reporting can be forgotten.
func TestHealthOneLinePerKindle(t *testing.T) {
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.DataDir = t.TempDir() })
	admin := login(t, srv, "admin", "adminpass1")
	u, _ := st.UserByName("admin")
	for _, d := range []string{"Old Kindle (Amazon reader)", "New Kindle (Amazon reader)"} {
		_, _ = st.SaveDeviceBooks(u.ID, d, []store.DeviceBook{{Filename: "x.epub", Title: "X"}})
		st.SaveDeviceInfo(u.ID, d, "app 1.0.0")
	}
	_, _ = st.DB.Exec(`UPDATE device_info SET reported_at = ? WHERE device = 'Old Kindle (Amazon reader)'`, time.Now().Add(-20*24*time.Hour).Unix())
	_, h := admin.do("GET", "/api/admin/health", nil, false)
	var kindles []map[string]any
	for _, it := range h["items"].([]any) {
		m := it.(map[string]any)
		if strings.Contains(m["title"].(string), "Kindle") {
			kindles = append(kindles, m)
		}
	}
	if len(kindles) != 2 {
		t.Fatalf("one line per Kindle, not one per problem: %v", kindles)
	}
	if res, _ := admin.do("POST", "/api/admin/devices/forget", map[string]any{"user_id": u.ID, "device": "New Kindle (Amazon reader)"}, true); res.StatusCode != 409 {
		t.Fatalf("a Kindle that just reported can't be forgotten: %d", res.StatusCode)
	}
	if res, out := admin.do("POST", "/api/admin/devices/forget", map[string]any{"user_id": u.ID, "device": "Old Kindle (Amazon reader)"}, true); res.StatusCode != 200 {
		t.Fatalf("forget: %d %v", res.StatusCode, out)
	}
	if got := len(st.AllDevices()); got != 1 {
		t.Fatalf("one Kindle left: %d", got)
	}
}

// /kindleapp (in any capitals) is a page anyone can open; the zip behind it is made for whoever signed in.
func TestKindleAppPage(t *testing.T) {
	srv, _ := setupWith(t, func(s *api.Server) {
		s.Web = fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}, "kindleapp.html": {Data: []byte("<title>NovelCheck for your Kindle</title>")}}
	})
	for _, p := range []string{"/kindleapp", "/Kindleapp", "/KindleApp/"} {
		res, err := http.Get(srv.URL + p)
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("%s: %v %v", p, err, res)
		}
		b := make([]byte, 400)
		n, _ := res.Body.Read(b)
		res.Body.Close()
		if !strings.Contains(string(b[:n]), "NovelCheck for your Kindle") {
			t.Fatalf("%s isn't the Kindle app page: %q", p, b[:n])
		}
	}
	admin := login(t, srv, "admin", "adminpass1")
	req, _ := http.NewRequest("GET", srv.URL+"/api/me/kindle-app.zip?name=Jenna%27s+Paperwhite", nil)
	res, err := admin.http.Do(req)
	if err != nil || res.StatusCode != 200 || !strings.Contains(res.Header.Get("Content-Type"), "zip") {
		t.Fatalf("the zip: %v %v", err, res)
	}
	res.Body.Close()
	if res, _ := http.Get(srv.URL + "/api/me/kindle-app.zip"); res.StatusCode != 401 {
		t.Fatalf("the zip needs a sign-in: %d", res.StatusCode)
	}
}

// A new NovelCheck's admin sees what to do first, and "Hide" (a week's snooze) takes it away; kids never see it.
func TestHomeGettingStarted(t *testing.T) {
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	_, home := admin.do("GET", "/api/home", nil, false)
	steps, _ := home["setup"].([]any)
	if len(steps) < 4 {
		t.Fatalf("the admin's list: %v", home["setup"])
	}
	first, _ := steps[0].(map[string]any)
	if first["id"] != "books" || first["done"] != false || first["link"] == "" {
		t.Fatalf("a fresh library has no books yet: %v", first)
	}
	admin.do("POST", "/api/home/snooze", map[string]any{"id": "setup"}, true)
	if _, home = admin.do("GET", "/api/home", nil, false); home["setup"] != nil {
		t.Fatalf("hidden: %v", home["setup"])
	}
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid2", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid2", "kidpass12")
	if _, kh := kid.do("GET", "/api/home", nil, false); kh["setup"] != nil {
		t.Fatalf("kids never see the list: %v", kh["setup"])
	}
}

// Autopilot starts off, an admin turns it on from Home, and kids cannot touch it.
func TestAutopilotSwitch(t *testing.T) {
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	if _, r := admin.do("GET", "/api/admin/autopilot", nil, false); r["on"] != false {
		t.Fatalf("starts off: %v", r)
	}
	if _, r := admin.do("PUT", "/api/admin/autopilot", map[string]any{"on": true}, true); r["on"] != true {
		t.Fatalf("turned on: %v", r)
	}
	_, home := admin.do("GET", "/api/home", nil, false)
	if a, _ := home["autopilot"].(map[string]any); a == nil || a["on"] != true {
		t.Fatalf("Home says so: %v", home["autopilot"])
	}
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid3", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid3", "kidpass12")
	if res, _ := kid.do("PUT", "/api/admin/autopilot", map[string]any{"on": false}, true); res.StatusCode != 403 {
		t.Fatalf("kids: %d", res.StatusCode)
	}
}

// Parents see how far the AI's levels can be trusted on Home, once enough books were checked by people.
func TestHomeShowsAccuracy(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	if _, h := admin.do("GET", "/api/home", nil, false); h["accuracy"] != nil {
		t.Fatalf("too few checked books: %v", h["accuracy"])
	}
	var uid int64
	_ = st.DB.Get(&uid, `SELECT id FROM users WHERE username = 'admin'`)
	for i := 0; i < 6; i++ {
		b, _ := st.UpsertBook("Checked "+string(rune('A'+i)), "Au Thor", "", "")
		level, ai := 2, 2
		if i == 0 {
			ai = 4 // one the AI got wrong by two levels
		}
		if _, err := st.DB.Exec(`INSERT INTO rating_feedback (book_id, user_id, level, ai_level, model) VALUES (?, ?, ?, ?, 'm')`, b, uid, level, ai); err != nil {
			t.Fatal(err)
		}
	}
	_, h := admin.do("GET", "/api/home", nil, false)
	a, _ := h["accuracy"].(map[string]any)
	if a == nil || a["n"] != float64(6) || a["exact"] != float64(83) || a["within1"] != float64(83) {
		t.Fatalf("accuracy: %v", h["accuracy"])
	}
}

// One Kindle reported by two programs (the KOReader plugin and the Kindle app, linked as the same Kindle) is one line on
// Health, not two; an unnamed app and the only plugin of the same person are linked by themselves.
func TestOneKindleTwoReportsIsOneLine(t *testing.T) {
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.DataDir = t.TempDir() })
	admin := login(t, srv, "admin", "adminpass1")
	u, _ := st.UserByName("admin")
	st.SaveDeviceInfo(u.ID, "Zach Paperwhite", "1.0.0")
	st.SaveDeviceInfo(u.ID, "Kindle (Amazon reader)", "app 1.0.0")
	st.AutoLink(u.ID, "Kindle (Amazon reader)") // what a report does
	_, h := admin.do("GET", "/api/admin/health", nil, false)
	n := 0
	for _, it := range h["items"].([]any) {
		if strings.Contains(it.(map[string]any)["title"].(string), "Paperwhite") || strings.Contains(it.(map[string]any)["title"].(string), "Kindle (Amazon") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("one Kindle, one line: %v", h["items"])
	}
}

// A backed-up Kindle that only runs an older app is an FYI on the Health page, and is left off Home.
func TestOlderAppIsOnlyANote(t *testing.T) {
	dir := t.TempDir()
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.DataDir = dir })
	admin := login(t, srv, "admin", "adminpass1")
	u, _ := st.UserByName("admin")
	st.SaveDeviceInfo(u.ID, "Pat Kindle (Amazon reader)", "app 1.0.0")
	_, _ = st.SaveDeviceBooks(u.ID, "Pat Kindle (Amazon reader)", []store.DeviceBook{{Filename: "x.epub", Title: "X"}})
	if err := os.MkdirAll(filepath.Join(dir, "kindle-backup", fmt.Sprint(u.ID), "Pat Kindle (Amazon reader)"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "kindle-backup", fmt.Sprint(u.ID), "Pat Kindle (Amazon reader)", "book.epub"), []byte("x"), 0o644)
	_, h := admin.do("GET", "/api/admin/health", nil, false)
	level := ""
	for _, it := range h["items"].([]any) {
		if m := it.(map[string]any); strings.Contains(m["title"].(string), "Pat Kindle") {
			level, _ = m["level"].(string)
		}
	}
	if level != "info" {
		t.Fatalf("an older app alone is an FYI, not a warning: %q %v", level, h["items"])
	}
	_, home := admin.do("GET", "/api/home", nil, false)
	if hh, _ := home["health"].(map[string]any); hh != nil {
		for _, it := range hh["items"].([]any) {
			if strings.Contains(it.(map[string]any)["title"].(string), "Pat Kindle") {
				t.Fatalf("Home must not show it: %v", hh)
			}
		}
	}
}

// "Running now": admins see what the server is doing; kids cannot ask.
func TestRunningNow(t *testing.T) {
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	_, r := admin.do("GET", "/api/admin/running", nil, false)
	if list, ok := r["running"].([]any); !ok || len(list) != 0 {
		t.Fatalf("nothing runs on a fresh server: %v", r)
	}
	if _, home := admin.do("GET", "/api/home", nil, false); home["running"] == nil {
		t.Fatalf("Home carries the list: %v", home)
	}
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid4", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid4", "kidpass12")
	if res, _ := kid.do("GET", "/api/admin/running", nil, false); res.StatusCode != 403 {
		t.Fatalf("kids: %d", res.StatusCode)
	}
}
