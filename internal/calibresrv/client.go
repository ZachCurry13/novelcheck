// Package calibresrv talks to a running calibre Content server so NovelCheck
// can remove books *through calibre* (the same remote interface calibredb
// uses). Calibre then updates its database and moves the files to its
// recycle bin; NovelCheck itself never writes to the library.
package calibresrv

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	URL      string // e.g. http://192.168.1.50:8081
	Username string // calibre user allowed to make changes
	Password string
	Library  string // library id; "" = server default
	HTTP     *http.Client
}

// Normalize validates and cleans a user-entered server address.
func Normalize(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("enter the Content server address, for example http://192.168.1.50:8081")
	}
	u.Path, u.RawQuery, u.Fragment = strings.TrimSuffix(u.Path, "/"), "", ""
	return strings.TrimSuffix(u.String(), "/"), nil
}

func (c *Client) httpc() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second}
}

// do sends a request, answering a Basic or Digest login challenge if the
// server asks for one.
func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	send := func(auth string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, method, c.URL+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		return c.httpc().Do(req)
	}
	resp, err := send("")
	if err != nil {
		return nil, fmt.Errorf("can't reach the calibre Content server: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized && c.Username != "" {
		challenge := resp.Header.Get("WWW-Authenticate")
		resp.Body.Close()
		auth, err := c.authorize(challenge, method, path)
		if err != nil {
			return nil, err
		}
		if resp, err = send(auth); err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return data, nil
	case http.StatusUnauthorized:
		return nil, errors.New("calibre didn't accept the username or password")
	case http.StatusForbidden:
		return nil, fmt.Errorf("calibre refused: %s", strings.TrimSpace(string(data)))
	case http.StatusNotFound:
		return nil, fmt.Errorf("calibre says not found: %s", strings.TrimSpace(string(data)))
	}
	return nil, fmt.Errorf("calibre answered %s", resp.Status)
}

// authorize builds the Authorization header for a Basic or Digest challenge.
func (c *Client) authorize(challenge, method, uri string) (string, error) {
	scheme, params, _ := strings.Cut(challenge, " ")
	switch strings.ToLower(scheme) {
	case "basic":
		req, _ := http.NewRequest(http.MethodGet, "/", nil)
		req.SetBasicAuth(c.Username, c.Password)
		return req.Header.Get("Authorization"), nil
	case "digest":
		p := parseParams(params)
		if alg := strings.ToUpper(p["algorithm"]); alg != "" && alg != "MD5" {
			return "", fmt.Errorf("unsupported calibre login method %q", alg)
		}
		cnonce := make([]byte, 8)
		_, _ = rand.Read(cnonce)
		cn, nc := hex.EncodeToString(cnonce), "00000001"
		ha1 := md5hex(c.Username + ":" + p["realm"] + ":" + c.Password)
		ha2 := md5hex(method + ":" + uri)
		var resp string
		qop := ""
		if strings.Contains(p["qop"], "auth") {
			qop = "auth"
			resp = md5hex(strings.Join([]string{ha1, p["nonce"], nc, cn, qop, ha2}, ":"))
		} else {
			resp = md5hex(ha1 + ":" + p["nonce"] + ":" + ha2)
		}
		h := fmt.Sprintf(`Digest username=%q, realm=%q, nonce=%q, uri=%q, algorithm=MD5, response=%q`,
			c.Username, p["realm"], p["nonce"], uri, resp)
		if qop != "" {
			h += fmt.Sprintf(`, qop=%s, nc=%s, cnonce=%q`, qop, nc, cn)
		}
		if p["opaque"] != "" {
			h += fmt.Sprintf(`, opaque=%q`, p["opaque"])
		}
		return h, nil
	}
	return "", fmt.Errorf("unsupported calibre login method %q", scheme)
}

func md5hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// parseParams parses `a="x", b=y` challenge parameters.
func parseParams(s string) map[string]string {
	out := map[string]string{}
	for len(s) > 0 {
		s = strings.TrimLeft(s, " ,")
		key, rest, ok := strings.Cut(s, "=")
		if !ok {
			break
		}
		var val string
		if strings.HasPrefix(rest, `"`) {
			end := strings.Index(rest[1:], `"`)
			if end < 0 {
				break
			}
			val, s = rest[1:end+1], rest[end+2:]
		} else {
			val, s, _ = strings.Cut(rest, ",")
		}
		out[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(val)
	}
	return out
}

// Libraries returns the server's library ids and its default library.
func (c *Client) Libraries(ctx context.Context) (map[string]string, string, error) {
	data, err := c.do(ctx, http.MethodGet, "/ajax/library-info", nil)
	if err != nil {
		return nil, "", err
	}
	var info struct {
		LibraryMap map[string]string `json:"library_map"`
		Default    string            `json:"default_library"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, "", errors.New("that address doesn't look like a calibre Content server")
	}
	return info.LibraryMap, info.Default, nil
}

// cmd runs a calibredb command through the server.
func (c *Client) cmd(ctx context.Context, name string, args any) (json.RawMessage, error) {
	body, _ := json.Marshal(args)
	path := "/cdb/cmd/" + name + "/0"
	if c.Library != "" {
		path += "?library_id=" + url.QueryEscape(c.Library)
	}
	data, err := c.do(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	var out struct {
		Result json.RawMessage `json:"result"`
		Err    string          `json:"err"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, errors.New("unexpected answer from calibre")
	}
	if out.Err != "" {
		return nil, fmt.Errorf("calibre: %s", out.Err)
	}
	return out.Result, nil
}
