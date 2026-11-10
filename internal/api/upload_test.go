package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func multipartBody(fields [][2]string, files [][2]string) (*bytes.Buffer, string) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, f := range fields {
		_ = mw.WriteField(f[0], f[1])
	}
	for i, f := range files {
		name := "file"
		if len(fields) > 0 {
			name = string(rune('0' + i))
		}
		p, _ := mw.CreateFormFile(name, f[0])
		p.Write([]byte(f[1]))
	}
	mw.Close()
	return &buf, mw.FormDataContentType()
}

func postForm(t *testing.T, c *client, path string, body *bytes.Buffer, ctype string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest("POST", c.base+path, body)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("X-NovelCheck", "1")
	res, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

// An e-book goes into Calibre (and is read back); audiobook files are streamed to Audiobookshelf with the chosen
// library and folder; wrong file types are refused.
func TestUploads(t *testing.T) {
	cal := &fakeCalibreDetails{book: map[string]any{"title": "Cinder", "authors": "Marissa Meyer", "series": "", "series_index": 1.0,
		"tags": []string{}, "comments": "", "identifiers": map[string]string{}}}
	cs := cal.serve(t)
	var absFields map[string]string
	var absFiles []string
	abs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/libraries":
			io.WriteString(w, `{"libraries":[{"id":"lib1","name":"Audiobooks","mediaType":"book"}]}`)
		case r.URL.Path == "/api/libraries/lib1":
			io.WriteString(w, `{"folders":[{"id":"fol1","fullPath":"/audiobooks"}]}`)
		case r.URL.Path == "/api/upload" && r.Method == "POST":
			absFields, absFiles = map[string]string{}, nil
			mr, _ := r.MultipartReader()
			for {
				p, err := mr.NextPart()
				if err != nil {
					break
				}
				b, _ := io.ReadAll(p)
				if p.FileName() != "" {
					absFiles = append(absFiles, p.FileName()+":"+string(b))
				} else {
					absFields[p.FormName()] = string(b)
				}
			}
			io.WriteString(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer abs.Close()
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")

	body, ct := multipartBody(nil, [][2]string{{"Cinder.epub", "EPUBDATA"}})
	if code, _ := postForm(t, admin, "/api/admin/upload/ebook", body, ct); code != 400 {
		t.Fatalf("needs the Content server first: %d", code)
	}
	admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": cs.URL}, true)
	body, ct = multipartBody(nil, [][2]string{{"Cinder.epub", "EPUBDATA"}})
	code, out := postForm(t, admin, "/api/admin/upload/ebook", body, ct)
	if code != 200 || out["calibre_id"] != float64(5) || out["title"] != "Cinder" {
		t.Fatalf("e-book upload: %d %v", code, out)
	}
	if len(cal.uploads) != 1 || !strings.Contains(cal.uploads[0], "/cdb/add-book/0/n/Cinder.epub/books application/octet-stream 8") {
		t.Fatalf("calibre got: %v", cal.uploads)
	}
	body, ct = multipartBody(nil, [][2]string{{"virus.exe", "MZ"}})
	if code, _ := postForm(t, admin, "/api/admin/upload/ebook", body, ct); code != 400 || len(cal.uploads) != 1 {
		t.Fatalf("only e-book files: %d", code)
	}

	_ = st.SetSetting(store.KeyABSURL, abs.URL)
	_ = st.SetSetting(store.KeyABSToken, "tok")
	_, tg := admin.do("GET", "/api/admin/upload/targets", nil, false)
	libs, _ := tg["abs"].([]any)
	if len(libs) != 1 || libs[0].(map[string]any)["folders"].([]any)[0].(map[string]any)["id"] != "fol1" {
		t.Fatalf("targets: %v", tg)
	}
	fields := [][2]string{{"title", "The Hobbit"}, {"author", "J. R. R. Tolkien"}, {"library", "lib1"}, {"folder", "fol1"}}
	body, ct = multipartBody(fields, [][2]string{{"01.mp3", "AUDIO1"}, {"02.mp3", "AUDIO2"}})
	code, out = postForm(t, admin, "/api/admin/upload/audiobook", body, ct)
	if code != 200 || out["sent"] != "The Hobbit" {
		t.Fatalf("audiobook upload: %d %v", code, out)
	}
	if absFields["library"] != "lib1" || absFields["folder"] != "fol1" || absFields["author"] != "J. R. R. Tolkien" || len(absFiles) != 2 || absFiles[1] != "02.mp3:AUDIO2" {
		t.Fatalf("audiobookshelf got: %v %v", absFields, absFiles)
	}
	body, ct = multipartBody([][2]string{{"title", "X"}}, [][2]string{{"a.mp3", "A"}})
	if code, _ := postForm(t, admin, "/api/admin/upload/audiobook", body, ct); code != 400 {
		t.Fatalf("a library and folder are required: %d", code)
	}
}
