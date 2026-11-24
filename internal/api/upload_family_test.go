package api_test

import (
	"encoding/json"

	"bytes"
	"github.com/zachcurry13/novelcheck/internal/api"
	"mime/multipart"
	"net/http"
	"testing"
)

func uploadStatus(t *testing.T, c *client) int {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "x.epub")
	_, _ = fw.Write([]byte("PK"))
	_ = mw.Close()
	req, _ := http.NewRequest("POST", c.base+"/api/upload/ebook", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-NovelCheck", "1")
	res, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

// Only admins and people an admin switched on may add books; the rest are refused before anything is read.
func TestOnlyAllowedPeopleAddBooks(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	if got := uploadStatus(t, kid); got != http.StatusForbidden {
		t.Fatalf("a person not switched on got %d", got)
	}
	u, _ := st.UserByName("kid")
	admin.do("PUT", "/api/admin/users/"+itoa(u.ID), map[string]any{"role": u.Role, "can_upload": true}, true)
	if got := uploadStatus(t, kid); got != http.StatusBadRequest { // allowed, but Calibre isn't set up in the test
		t.Fatalf("an allowed person got %d", got)
	}
}

func uploadJSON(t *testing.T, c *client) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "cinder.epub")
	_, _ = fw.Write([]byte("PK"))
	_ = mw.Close()
	req, _ := http.NewRequest("POST", c.base+"/api/upload/ebook", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-NovelCheck", "1")
	res, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	if res.StatusCode != 200 {
		t.Fatalf("upload: %d %v", res.StatusCode, out)
	}
	return out
}

// A sure Hardcover match fixes the book in Calibre; a different book behind the ISBN is only offered as a guess.
func TestUploadIsCheckedWithHardcover(t *testing.T) {
	api.SetHardcoverURL(cleanupHardcover(t).URL)
	t.Cleanup(func() { api.SetHardcoverURL("") })
	cal := &fakeCalibreDetails{book: map[string]any{
		"title": "Cinder (The Lunar Chronicles Book 1)", "authors": "Unknown", "series": "", "series_index": 1.0,
		"tags": []string{}, "comments": "", "identifiers": map[string]string{"isbn": "9780312641894"}, "cover": "/c/cover.jpg",
	}}
	cs := cal.serve(t)
	srv, _ := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("PUT", "/api/me/hardcover", map[string]string{"token": "good-token"}, true)
	admin.do("PUT", "/api/admin/calibre/server", map[string]string{"url": cs.URL}, true)

	r := uploadJSON(t, admin)
	if r["status"] != "matched" {
		t.Fatalf("a sure match: %v", r)
	}
	if cal.book["title"] != "Cinder" || cal.book["authors"] != "Marissa Meyer" || cal.book["series"] != "The Lunar Chronicles" {
		t.Fatalf("Calibre was not fixed: %v", cal.book)
	}

	cal.book["title"] = "Some Other Book"
	cal.book["authors"] = "Someone Else"
	r = uploadJSON(t, admin)
	g, _ := r["guess"].(map[string]any)
	if r["status"] != "unsure" || g["title"] != "Cinder" || cal.book["title"] != "Some Other Book" {
		t.Fatalf("a different book behind the ISBN must only be a guess: %v", r)
	}
}
