package api_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// A fake Audiobookshelf with two audiobooks: Dune (one 12h file) and Mistborn (two 20h files).
func fakeABS(t *testing.T) (*httptest.Server, *[]string) {
	var mu sync.Mutex
	var saved []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" && r.URL.Path != "/login" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		switch {
		case r.URL.Path == "/login":
			fmt.Fprint(w, `{"user": {"username": "pat", "token": "pat-token"}}`)
		case r.URL.Path == "/api/libraries":
			fmt.Fprint(w, `{"libraries": [{"id": "lib1", "name": "Audiobooks", "mediaType": "book"}, {"id": "pods", "name": "Pods", "mediaType": "podcast"}]}`)
		case r.URL.Path == "/api/libraries/lib1/items":
			fmt.Fprint(w, `{"total": 3, "results": [
			 {"id": "li_dune", "media": {"duration": 43200, "metadata": {"title": "Dune", "authorName": "Frank Herbert", "narratorName": "Scott Brick, Simon Vance",
			   "seriesName": "Dune #1", "genres": ["Science Fiction"], "description": "<p>A desert planet.</p>"}}},
			 {"id": "li_mist", "media": {"duration": 144000, "metadata": {"title": "Mistborn", "authorName": "Brandon Sanderson", "narratorName": "Michael Kramer"}}},
			 {"id": "li_ebook", "media": {"duration": 0, "metadata": {"title": "Only An Ebook", "authorName": "Nobody"}}}]}`)
		case r.URL.Path == "/api/items/li_mist":
			fmt.Fprint(w, `{"media": {"duration": 144000, "audioFiles": [
			 {"index": 2, "ino": "b", "duration": 72000, "mimeType": "audio/mpeg", "metadata": {"filename": "02.mp3"}},
			 {"index": 1, "ino": "a", "duration": 72000, "mimeType": "audio/mpeg", "metadata": {"filename": "01.mp3"}}],
			 "chapters": [{"title": "Prologue", "start": 0, "end": 600}]}}`)
		case r.URL.Path == "/api/items/li_dune":
			fmt.Fprint(w, `{"media": {"duration": 43200, "audioFiles": [{"index": 1, "ino": "d1", "duration": 43200, "mimeType": "audio/mp4", "metadata": {"filename": "dune.m4b"}}]}}`)
		case r.URL.Path == "/api/me":
			fmt.Fprint(w, `{"mediaProgress": [{"libraryItemId": "li_mist", "currentTime": 3600, "duration": 144000, "progress": 0.025, "isFinished": false, "lastUpdate": 5},
			 {"libraryItemId": "li_dune", "currentTime": 43000, "duration": 43200, "progress": 0.99, "isFinished": true, "lastUpdate": 9}],
			 "bookmarks": [{"libraryItemId": "li_mist", "time": 120, "title": "Good bit"}]}`)
		case strings.HasPrefix(r.URL.Path, "/api/me/progress/") && r.Method == http.MethodPatch:
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			saved = append(saved, r.URL.Path+" "+string(b))
			mu.Unlock()
		case r.URL.Path == "/api/items/li_mist/file/a":
			w.Header().Set("Content-Type", "audio/mpeg")
			w.Header().Set("Accept-Ranges", "bytes")
			if r.Header.Get("Range") != "" {
				w.Header().Set("Content-Range", "bytes 0-3/10")
				w.WriteHeader(http.StatusPartialContent)
			}
			fmt.Fprint(w, "ABCD")
		case r.URL.Path == "/api/items/li_dune/file/d1/download":
			w.Header().Set("Content-Type", "audio/mp4")
			fmt.Fprint(w, "M4B-BYTES")
		case r.URL.Path == "/api/items/li_mist/download":
			w.Header().Set("Content-Type", "application/zip")
			fmt.Fprint(w, "ZIP-BYTES")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &saved
}

func TestAudiobookshelf(t *testing.T) {
	abs, saved := fakeABS(t)
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")

	// an e-book of Dune is already in the library: the audiobook joins it
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	dune, _ := st.UpsertBook("Dune", "Frank Herbert", "", "")
	_ = st.AddCopy(cal, dune, "/c/dune.epub", "epub", "1")

	if res, _ := admin.do("PUT", "/api/admin/settings", map[string]string{"abs_url": "not a url ::"}, true); res.StatusCode != 400 {
		t.Fatalf("a bad address: %d", res.StatusCode)
	}
	admin.do("PUT", "/api/admin/settings", map[string]string{"abs_url": abs.URL + "/", "abs_api_token": "secret"}, true)
	if res, out := admin.do("POST", "/api/admin/audiobookshelf/test", map[string]string{"url": abs.URL, "token": "********"}, true); res.StatusCode != 200 || len(out["libraries"].([]any)) != 1 {
		t.Fatalf("test connection (podcast libraries are left out): %d %v", res.StatusCode, out)
	}
	if res, _ := admin.do("POST", "/api/admin/audiobookshelf/test", map[string]string{"url": abs.URL, "token": "wrong"}, true); res.StatusCode != 502 {
		t.Fatalf("a wrong token: %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", "/api/admin/audiobookshelf/sync", nil, true); res.StatusCode != 202 {
		t.Fatalf("sync: %d", res.StatusCode)
	}
	for i := 0; i < 100; i++ {
		if _, out := admin.do("GET", "/api/admin/audiobookshelf", nil, false); out["busy"] == false && out["audiobooks"].(float64) == 2 &&
			out["result"] != "" { // the result line is written just after the count
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	_, status := admin.do("GET", "/api/admin/audiobookshelf", nil, false)
	if status["audiobooks"].(float64) != 2 || !strings.Contains(status["result"].(string), "2 audiobooks") {
		t.Fatalf("status after the sync (the e-book-only entry is skipped): %v", status)
	}

	// sorted by length, filtered by length, and by format
	order := func(q string) string {
		_, out := admin.do("GET", "/api/books?format=audio&"+q, nil, false)
		var names []string
		for _, b := range out["books"].([]any) {
			names = append(names, b.(map[string]any)["title"].(string))
		}
		return strings.Join(names, ",")
	}
	if got := order("sort=long"); got != "Mistborn,Dune" {
		t.Fatalf("longest first: %s", got)
	}
	if got := order("sort=short"); got != "Dune,Mistborn" {
		t.Fatalf("shortest first: %s", got)
	}
	if got := order("max_hours=20"); got != "Dune" {
		t.Fatalf("under 20 hours: %s", got)
	}
	if got := order("min_hours=20"); got != "Mistborn" {
		t.Fatalf("over 20 hours: %s", got)
	}
	if got := order("narrator=kramer"); got != "Mistborn" {
		t.Fatalf("by narrator: %s", got)
	}

	// one book with two formats
	_, out := admin.do("GET", "/api/books/"+itoa(dune), nil, false)
	b := out["book"].(map[string]any)
	if out["has_audio"] != true || out["has_ebook"] != true || b["duration_sec"].(float64) != 43200 || b["narrator"] != "Scott Brick, Simon Vance" || b["series"] != "Dune" {
		t.Fatalf("the audiobook joined the e-book: %v", out)
	}
	if !strings.Contains(b["description"].(string), "desert") && b["description"] != "" {
		t.Fatalf("description: %v", b["description"])
	}

	// the player's data, saving progress, continue listening
	mist, _ := st.UpsertBook("Mistborn", "Brandon Sanderson", "", "")
	_, listen := admin.do("GET", "/api/books/"+itoa(mist)+"/listen", nil, false)
	d := listen["detail"].(map[string]any)
	tracks := d["tracks"].([]any)
	if len(tracks) != 2 || tracks[0].(map[string]any)["ino"] != "a" || tracks[1].(map[string]any)["start"].(float64) != 72000 ||
		len(d["bookmarks"].([]any)) != 1 || d["progress"].(map[string]any)["current_time"].(float64) != 3600 || len(d["chapters"].([]any)) != 1 {
		t.Fatalf("listen detail: %v", listen)
	}
	if res, _ := admin.do("PUT", "/api/books/"+itoa(mist)+"/listen/progress", map[string]any{"at": 5000, "duration": 144000}, true); res.StatusCode != 200 || len(*saved) != 1 || !strings.Contains((*saved)[0], `"currentTime":5000`) {
		t.Fatalf("progress is passed on: %v", *saved)
	}
	_, now := admin.do("GET", "/api/listening", nil, false)
	if bs := now["books"].([]any); len(bs) != 1 || bs[0].(map[string]any)["title"] != "Mistborn" {
		t.Fatalf("continue listening (finished books are left out): %v", now)
	}

	// streaming passes Range through; downloads are an attachment
	req, _ := http.NewRequest("GET", srv.URL+"/api/books/"+itoa(mist)+"/audio/a", nil)
	req.Header.Set("Range", "bytes=0-3")
	for _, c := range admin.http.Jar.Cookies(req.URL) {
		req.AddCookie(c)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 206 || string(body) != "ABCD" || res.Header.Get("Content-Range") == "" {
		t.Fatalf("stream: %d %q %v", res.StatusCode, body, res.Header)
	}
	if res, _ := admin.do("GET", "/api/books/"+itoa(mist)+"/audio/..%2fsecret", nil, false); res.StatusCode == 200 {
		t.Fatalf("a path trick must not stream")
	}
	dl := func(id int64) (*http.Response, string) {
		req, _ := http.NewRequest("GET", srv.URL+"/api/books/"+itoa(id)+"/download?format=audio", nil)
		for _, c := range admin.http.Jar.Cookies(req.URL) {
			req.AddCookie(c)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return res, string(b)
	}
	if res, body := dl(dune); body != "M4B-BYTES" || res.Header.Get("Content-Disposition") != `attachment; filename="Dune.m4b"` {
		t.Fatalf("one file is sent as it is: %q %v", body, res.Header)
	}
	if res, body := dl(mist); body != "ZIP-BYTES" || res.Header.Get("Content-Disposition") != `attachment; filename="Mistborn.zip"` {
		t.Fatalf("several files come as a zip: %q %v", body, res.Header)
	}

	// kids' rules apply to the audio too: an unrated book is closed to a kid
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	if res, _ := kid.do("GET", "/api/books/"+itoa(mist)+"/listen", nil, false); res.StatusCode != 404 {
		t.Fatalf("a kid can't listen to an unrated book: %d", res.StatusCode)
	}

	// each person can link their own account
	if res, _ := admin.do("PUT", "/api/me/audiobookshelf", map[string]string{"username": "pat", "password": "x"}, true); res.StatusCode != 200 {
		t.Fatalf("link: %d", res.StatusCode)
	}
	if tok, name := st.ABSLink(1); tok != "pat-token" || name != "pat" {
		t.Fatalf("the token is kept, not the password: %q %q", tok, name)
	}
	admin.do("DELETE", "/api/me/audiobookshelf", nil, true)
	if tok, _ := st.ABSLink(1); tok != "" {
		t.Fatalf("unlinked")
	}
}

// Wishing for the other format of a title the family already has.
func TestWishOtherFormat(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	id, _ := st.UpsertBook("Dune", "Frank Herbert", "", "")
	_ = st.AddCopy(cal, id, "/c/dune.epub", "epub", "1")
	path := "/api/books/" + itoa(id) + "/wish"
	if res, _ := admin.do("POST", path, map[string]string{"format": "ebook"}, true); res.StatusCode != 400 {
		t.Fatalf("it already has the e-book: %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", path, map[string]string{"format": "audio"}, true); res.StatusCode != 200 {
		t.Fatalf("ask for the audiobook: %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", path, map[string]string{"format": "audio"}, true); res.StatusCode != 409 {
		t.Fatalf("only once: %d", res.StatusCode)
	}
	_, out := admin.do("GET", "/api/books/"+itoa(id), nil, false)
	if out["my_audio_wish"] == nil || out["my_ebook_wish"] != nil {
		t.Fatalf("the wish shows on the book: %v", out)
	}
	_, list := admin.do("GET", "/api/wishlist", nil, false)
	if it := list["items"].([]any); len(it) != 1 || it[0].(map[string]any)["format"] != "audio" {
		t.Fatalf("the wishlist says which format: %v", list)
	}
	// the audiobook arrives: the wish is marked acquired
	audio, _ := st.EnsureCatalog(store.AudiobookshelfCatalog, "custom")
	_ = st.AddCopy(audio, id, "abs:x", "audio", "x")
	_, list = admin.do("GET", "/api/wishlist", nil, false)
	if list["items"].([]any)[0].(map[string]any)["status"] != "acquired" {
		t.Fatalf("acquired once the audiobook is in: %v", list)
	}
}
