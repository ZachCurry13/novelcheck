package api_test

import (
	"bytes"
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
