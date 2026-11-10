package audiobookshelf

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// UploadFile is one file of an audiobook (an .m4b, or the .mp3 parts, a cover).
type UploadFile struct {
	Name string
	R    io.Reader
}

// Upload sends an audiobook to a library folder the way the Audiobookshelf web page does (POST /api/upload, multipart
// with title, author, series, library and folder, then the files), streaming the files rather than holding them.
// UNVERIFIED against a real Audiobookshelf: see Admin → Check my servers.
func (c *Client) Upload(ctx context.Context, library, folder, title, author, series string, next func() (UploadFile, bool)) error {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		var err error
		for k, v := range map[string]string{"title": title, "author": author, "series": series, "library": library, "folder": folder} {
			if v == "" && k != "title" {
				continue
			}
			if err = mw.WriteField(k, v); err != nil {
				break
			}
		}
		for i := 0; err == nil; i++ {
			f, ok := next()
			if !ok {
				break
			}
			var part io.Writer
			if part, err = mw.CreateFormFile(fmt.Sprint(i), f.Name); err == nil {
				_, err = io.Copy(part, f.R)
			}
		}
		if err == nil {
			err = mw.Close()
		}
		pw.CloseWithError(err)
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Base+"/api/upload", pr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.client(true).Do(req)
	if err != nil {
		return fmt.Errorf("can't reach Audiobookshelf: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrSignedOut
	}
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("Audiobookshelf refused the upload (%s) %s", resp.Status, string(msg))
	}
	return nil
}
