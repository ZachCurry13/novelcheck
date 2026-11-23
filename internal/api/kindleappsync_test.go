package api_test

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The Kindle app's report: Amazon store books join a private store library (and get rated), Amazon's reader's
// progress counts as reading progress, the My Kindle list shows them, looked-up words reach the book's page, and the
// remote can remove a downloaded store book (never a sideloaded one) from that Kindle only.
func TestKindleAppSync(t *testing.T) {
	srv, st := setup(t)
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, ks := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	key := hex.EncodeToString(sum[:])
	call := func(method, path string, body any) (int, map[string]any) {
		var buf bytes.Buffer
		_ = json.NewEncoder(&buf).Encode(body)
		req, _ := http.NewRequest(method, srv.URL+path, &buf)
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", key)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		out := map[string]any{}
		_ = json.Unmarshal(raw, &out)
		return res.StatusCode, out
	}
	device := "Kindle (Amazon reader)"
	report := map[string]any{"device": device, "plugin_version": "app 1.2.0", "books": []map[string]any{
		{"md5": "asin:B0FOURTH", "filename": "Fourth Wing_B0FOURTH.kfx", "title": "Fourth Wing", "author": "Rebecca Yarros", "format": "kfx", "percent": 0.42, "opened_at": 1795000000},
		{"md5": "asin:B0CLOUD", "filename": "Iron Flame [B0CLOUD]", "title": "Iron Flame", "author": "Rebecca Yarros", "format": "cloud"},
		{"filename": "my-own.azw3", "title": "My Own Book", "author": "Someone", "format": "azw3"},
	}}
	if code, out := call("POST", "/kosync/plugin/device", report); code != 200 || out["matched"] != 2.0 {
		t.Fatalf("store books are added and matched, the sideloaded one isn't: %d %v", code, out)
	}
	fourth := st.MatchBook("Fourth Wing", "Rebecca Yarros")
	u, _ := st.UserByName("admin")
	if fourth == 0 || st.ReadingProgress(u.ID)[fourth].Percent != 0.42 {
		t.Fatalf("Amazon's reader's 42%% counts as reading progress: %d %v", fourth, st.ReadingProgress(u.ID)[fourth])
	}
	if c, _ := st.CatalogByName("admin's Kindle store"); c == nil || !c.Private {
		t.Fatalf("the store books are in a private library: %+v", c)
	}
	if code, out := call("POST", "/kosync/plugin/device", report); code != 200 || out["matched"] != 2.0 {
		t.Fatalf("a second report adds nothing twice: %v", out)
	}
	if _, out := call("GET", "/kosync/plugin/kindle-mine?device="+strings.ReplaceAll(device, " ", "%20"), nil); len(out["books"].([]any)) != 3 {
		t.Fatalf("My Kindle lists all three: %v", out)
	}
	words := map[string]any{"books": []map[string]any{{"title": "Fourth Wing", "author": "Rebecca Yarros", "words": []map[string]string{{"word": "wyvern", "usage": "A wyvern flew.", "at": "2026-11-20 21:00:00"}}}}}
	if _, out := call("POST", "/kosync/plugin/kindle-words", words); out["added"] != 1.0 {
		t.Fatalf("words: %v", out)
	}
	if _, out := call("POST", "/kosync/plugin/kindle-words", words); out["added"] != 0.0 {
		t.Fatalf("a word is kept once: %v", out)
	}
	_, hl := admin.do("GET", "/api/books/"+itoa(fourth)+"/highlights", nil, false)
	if w, _ := hl["words"].([]any); len(w) != 1 {
		t.Fatalf("the book's page has the word: %v", hl)
	}
	res, _ := admin.do("POST", "/api/me/kindle-tasks", map[string]any{"device": device, "kind": "remove_store", "filenames": []string{"my-own.azw3"}}, true)
	if res.StatusCode != 400 {
		t.Fatalf("a sideloaded book is never removed as a store book: %d", res.StatusCode)
	}
	res, _ = admin.do("POST", "/api/me/kindle-tasks", map[string]any{"device": device, "kind": "remove_store", "filenames": []string{"Fourth Wing_B0FOURTH.kfx", "Iron Flame [B0CLOUD]"}}, true)
	if res.StatusCode != 200 {
		t.Fatalf("a downloaded store book can be removed: %d", res.StatusCode)
	}
	if _, out := call("GET", "/kosync/plugin/tasks?device="+strings.ReplaceAll(device, " ", "%20"), nil); !strings.Contains(fmtJSON(out), `"filenames":["Fourth Wing_B0FOURTH.kfx"]`) {
		t.Fatalf("only the downloaded one is in the task: %v", out)
	}

	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	kidUser, _ := st.UserByName("kid")
	kid := login(t, srv, "kid", "kidpass12")
	if res, _ := kid.do("GET", "/api/me/kindle-tasks?user="+itoa(u.ID), nil, false); res.StatusCode != 403 {
		t.Fatalf("a kid can't use the remote on a parent's Kindle: %d", res.StatusCode)
	}
	if res, _ := admin.do("GET", "/api/me/kindle-tasks?user="+itoa(kidUser.ID), nil, false); res.StatusCode != 200 {
		t.Fatalf("a parent can use it on a kid's Kindle: %d", res.StatusCode)
	}
}

func fmtJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
