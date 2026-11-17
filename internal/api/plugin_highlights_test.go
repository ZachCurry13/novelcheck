package api_test

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Highlights and notes from the e-reader are matched to the library book, kept once, and shown on the book's page;
// finishing a book there gives its stars and the finished mark, and offers the next book of the series (unless it
// is already on the device).
func TestPluginHighlightsAndFinish(t *testing.T) {
	lib := t.TempDir()
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.CalibreDir = lib })
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, ks := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	key := hex.EncodeToString(sum[:])
	post := func(path, body string) (int, map[string]any) {
		req, _ := http.NewRequest("POST", srv.URL+"/kosync/plugin/"+path, strings.NewReader(body))
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", key)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		out := map[string]any{}
		_ = json.NewDecoder(res.Body).Decode(&out)
		return res.StatusCode, out
	}
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	mk := func(title string, n float64) int64 {
		id, _ := st.UpsertBook(title, "Brandon Sanderson", "", "")
		path := filepath.Join(lib, title+".epub")
		_ = os.WriteFile(path, []byte("epub"), 0o644)
		_ = st.AddCopy(cal, id, path, "epub", title)
		_, _ = st.DB.Exec(`UPDATE books SET series = 'Mistborn', series_index = ? WHERE id = ?`, n, id)
		return id
	}
	one, two := mk("The Final Empire", 1), mk("The Well of Ascension", 2)
	u, _ := st.UserByName("admin")

	body := `{"device": "Kindle", "books": [
		{"md5": "aa11", "filename": "empire.epub", "title": "The Final Empire", "author": "Brandon Sanderson", "highlights": [
			{"text": "Always there is another secret.", "note": "the theme", "chapter": "Chapter 1", "page": "12", "datetime": "2026-11-12 21:04:11"},
			{"text": "Trust in me.", "note": "", "chapter": "Chapter 3", "page": "40", "datetime": "2026-11-12 21:30:00"}]},
		{"md5": "zz99", "filename": "unknown.epub", "title": "Not In The Library", "author": "Nobody", "highlights": [{"text": "x", "datetime": "2026-11-12 22:00:00"}]}]}`
	if code, out := post("highlights", body); code != 200 || out["books"] != 1.0 || out["added"] != 2.0 || out["unmatched"] != 1.0 || fmt.Sprint(out["ok"]) != "[true false]" {
		t.Fatalf("first send: %d %v", code, out)
	}
	if _, out := post("highlights", strings.Replace(body, `"note": "the theme"`, `"note": "the theme, again"`, 1)); out["added"] != 0.0 {
		t.Fatalf("the same highlights are not added twice: %v", out)
	}
	_, got := admin.do("GET", "/api/books/"+itoa(one)+"/highlights", nil, false)
	list := got["highlights"].([]any)
	if len(list) != 2 || list[0].(map[string]any)["text"] != "Always there is another secret." || list[0].(map[string]any)["note"] != "the theme, again" {
		t.Fatalf("on the book's page, a note changed on the device updated: %v", list)
	}

	code, out := post("finish", `{"device": "Kindle", "md5": "aa11", "filename": "empire.epub", "title": "The Final Empire", "author": "Brandon Sanderson", "stars": 4.5, "day": "2026-11-13"}`)
	if code != 200 || out["stars"] != 4.5 {
		t.Fatalf("finish: %d %v", code, out)
	}
	if st.RatingOf(u.ID, one) != 4.5 || st.FinishedOn(u.ID, one) != "2026-11-13" {
		t.Fatalf("stars and the finished day are kept: %v %q", st.RatingOf(u.ID, one), st.FinishedOn(u.ID, one))
	}
	next, _ := out["next"].(map[string]any)
	if next["title"] != "The Well of Ascension" || next["have"] != true || next["id"] != float64(two) {
		t.Fatalf("the next book of the series: %v", out["next"])
	}
	// With the next book already on the device, nothing is offered.
	if _, err := st.SaveDeviceBooks(u.ID, "Kindle", []store.DeviceBook{{Filename: "well.epub", MD5: "bb22", Title: "The Well of Ascension", Author: "Brandon Sanderson"}}); err != nil {
		t.Fatal(err)
	}
	if _, out := post("finish", `{"device": "Kindle", "md5": "aa11", "filename": "empire.epub", "title": "The Final Empire", "author": "Brandon Sanderson"}`); out["next"] != nil {
		t.Fatalf("already on the device: %v", out["next"])
	}
	if code, _ := post("finish", `{"device": "Kindle", "md5": "zz99", "filename": "unknown.epub", "title": "Not In The Library", "author": "Nobody"}`); code != 404 {
		t.Fatalf("a book that isn't in the library: %d", code)
	}
}
