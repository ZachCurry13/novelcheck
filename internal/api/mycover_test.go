package api_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"testing"
)

func pngOf(c color.Color) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 120, 180))
	for x := 0; x < 120; x++ {
		for y := 0; y < 180; y++ {
			if (x/10+y/10)%2 == 0 { // a pattern: a plain one-color picture counts as "no image"
				img.Set(x, y, c)
			} else {
				img.Set(x, y, color.RGBA{uint8(x * 2), uint8(y), 90, 255})
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// Change cover: your upload changes only your view; "for everyone" is for admins; Back to the usual cover undoes it.
func TestChangeCover(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("POST", "/api/admin/users", map[string]any{"username": "sister", "password": "sisterpass1", "role": "editor"}, true)
	sister := login(t, srv, "sister", "sisterpass1")
	id, _ := st.UpsertBook("Cover Book", "A. Writer", "", "")
	cal, _ := st.EnsureCatalog("Family", "custom")
	_ = st.AddCopy(cal, id, "x", "list", "")
	upload := func(c *client, img []byte, everyone string) int {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("image", "mine.png")
		_, _ = fw.Write(img)
		_ = mw.WriteField("everyone", everyone)
		_ = mw.Close()
		req, _ := http.NewRequest("POST", srv.URL+"/api/books/"+itoa(id)+"/cover", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("X-NovelCheck", "1")
		res, err := c.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	cover := func(c *client) string {
		res, err := c.http.Get(srv.URL + "/api/books/" + itoa(id) + "/cover")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.Header.Get("Content-Type")
	}
	if cover(sister) != "image/svg+xml" {
		t.Fatal("no cover yet: the drawn stand-in")
	}
	if code := upload(sister, pngOf(color.RGBA{200, 0, 0, 255}), "false"); code != 200 {
		t.Fatalf("her upload: %d", code)
	}
	if cover(sister) != "image/jpeg" || cover(admin) != "image/svg+xml" {
		t.Fatal("her cover shows for her only")
	}
	if code := upload(sister, pngOf(color.Black), "true"); code != 403 {
		t.Fatalf("only an admin sets it for everyone: %d", code)
	}
	if code := upload(admin, pngOf(color.White), "true"); code != 200 || cover(admin) != "image/jpeg" {
		t.Fatalf("an admin's cover for everyone: %d", code)
	}
	if res, _ := sister.do("DELETE", "/api/books/"+itoa(id)+"/cover", nil, true); res.StatusCode != 200 {
		t.Fatal("back to the usual cover")
	}
	if code := upload(sister, []byte("not a picture at all, just some words that are long enough to pass the size check......................................................................................................................................................"), "false"); code != 400 {
		t.Fatalf("not a picture: %d", code)
	}
}
