// Package audiobookshelf talks to an Audiobookshelf server (self-hosted audiobook
// library): it lists the audiobooks, fetches covers, streams and downloads the
// audio, and keeps each person's listening progress and bookmarks. The server's
// address and token stay on the NovelCheck server; browsers only ever talk to NovelCheck.
package audiobookshelf

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is one connection: the server's address and an API token.
type Client struct {
	Base  string
	Token string
	HTTP  *http.Client // nil = a default with a timeout; streams use a client without one
}

// ErrSignedOut means the server no longer accepts the token.
var ErrSignedOut = errors.New("Audiobookshelf didn't accept the sign-in: link your account again (or check the API token)")

// CleanBase turns what was typed into the server's address ("" if it isn't one).
func CleanBase(s string) string {
	s = strings.TrimRight(strings.TrimSpace(s), "/")
	if s != "" && !strings.Contains(s, "://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return u.Scheme + "://" + u.Host + strings.TrimRight(u.Path, "/")
}

func (c *Client) client(stream bool) *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	if stream {
		return &http.Client{}
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// Do sends a request; the caller closes the body. header carries a Range for streams.
func (c *Client) Do(ctx context.Context, method, path string, body any, header http.Header, stream bool) (*http.Response, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, rd)
	if err != nil {
		return nil, err
	}
	for _, h := range []string{"Range", "If-Range"} {
		if v := header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.client(stream).Do(req)
	if err != nil {
		return nil, fmt.Errorf("can't reach Audiobookshelf: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		resp.Body.Close()
		return nil, ErrSignedOut
	}
	return resp, nil
}

// JSON sends a request and decodes the answer into out (nil to ignore it).
func (c *Client) JSON(ctx context.Context, method, path string, body, out any) error {
	resp, err := c.Do(ctx, method, path, body, nil, false)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("Audiobookshelf answered %s", resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(out)
}

// Login signs in once and returns the person's own token and name; the
// password isn't kept.
func Login(ctx context.Context, base, user, pass string) (token, name string, err error) {
	var out struct {
		User struct {
			Username    string `json:"username"`
			Token       string `json:"token"`
			AccessToken string `json:"accessToken"`
		} `json:"user"`
	}
	c := &Client{Base: base}
	resp, err := c.Do(ctx, http.MethodPost, "/login", map[string]string{"username": user, "password": pass}, nil, false)
	if err != nil {
		if errors.Is(err, ErrSignedOut) {
			return "", "", errors.New("Audiobookshelf says that name or password isn't right")
		}
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out) != nil {
		return "", "", fmt.Errorf("Audiobookshelf answered %s", resp.Status)
	}
	token = out.User.Token
	if token == "" {
		token = out.User.AccessToken
	}
	return token, out.User.Username, nil
}

// Cover fetches an item's cover as a JPEG.
func (c *Client) Cover(ctx context.Context, id string) ([]byte, error) {
	resp, err := c.Do(ctx, http.MethodGet, "/api/items/"+url.PathEscape(id)+"/cover?width=600&format=jpeg", nil, nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("no cover (%s)", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}
