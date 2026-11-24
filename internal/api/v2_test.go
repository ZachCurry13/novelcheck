package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
