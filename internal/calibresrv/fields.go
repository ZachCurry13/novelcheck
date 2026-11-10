package calibresrv

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Details is what calibre has for a book, for comparing with another source.
type Details struct {
	Title       string
	Authors     []string
	Series      string
	SeriesIndex float64
	Tags        []string
	Comments    string
	Identifiers map[string]string
}

// text reads a calibre value that may be a string, a number or null.
func text(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return ""
}

// list reads a calibre value that may be a list or one string split at sep.
func list(raw json.RawMessage, sep string) []string {
	var l []string
	if json.Unmarshal(raw, &l) == nil {
		return l
	}
	var s string
	if json.Unmarshal(raw, &s) == nil && strings.TrimSpace(s) != "" {
		var out []string
		for _, p := range strings.Split(s, sep) {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return nil
}

// Details returns calibre's title, authors, series, tags, comments and identifiers for each of ids (ids calibre
// doesn't have are absent), in one request.
func (c *Client) Details(ctx context.Context, ids []int) (map[int]Details, error) {
	if len(ids) == 0 {
		return map[int]Details{}, nil
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = "id:" + strconv.Itoa(id)
	}
	fields := []string{"title", "authors", "series", "series_index", "tags", "comments", "identifiers"}
	raw, err := c.cmd(ctx, "list", []any{fields, "id", true, strings.Join(parts, " or "), -1})
	if err != nil {
		return nil, err
	}
	var res struct {
		Data map[string]map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, errors.New("unexpected answer from calibre's book list")
	}
	out := map[int]Details{}
	for k, v := range res.Data["title"] {
		id, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		d := Details{Title: text(v), Authors: list(res.Data["authors"][k], "&"), Series: text(res.Data["series"][k]),
			Tags: list(res.Data["tags"][k], ","), Comments: text(res.Data["comments"][k]), Identifiers: map[string]string{}}
		d.SeriesIndex, _ = strconv.ParseFloat(text(res.Data["series_index"][k]), 64)
		var idm map[string]string
		if json.Unmarshal(res.Data["identifiers"][k], &idm) == nil {
			d.Identifiers = idm
		} else if s := text(res.Data["identifiers"][k]); s != "" { // "isbn:123,amazon:B00X"
			for _, p := range strings.Split(s, ",") {
				if kv := strings.SplitN(strings.TrimSpace(p), ":", 2); len(kv) == 2 {
					d.Identifiers[kv[0]] = kv[1]
				}
			}
		}
		out[id] = d
	}
	return out, nil
}

// SetFields changes fields of one book the way SetMetadata does ([field, value] pairs, calibre's own field
// names: title, authors (a list), series, series_index, tags (a list), comments, identifiers (a map)).
func (c *Client) SetFields(ctx context.Context, id int, fields [][]any) (found bool, err error) {
	raw, err := c.cmd(ctx, "set_metadata", []any{"fields", id, fields})
	if err != nil {
		return false, err
	}
	return len(raw) > 0 && string(raw) != "null", nil
}

// AddBook uploads a book file to calibre (like Add books in calibre): duplicates are refused, not added twice.
// It returns the new book's number. UNVERIFIED against a real calibre: see Admin → Check my servers.
func (c *Client) AddBook(ctx context.Context, filename string, data []byte) (int, error) {
	lib := c.Library
	if lib == "" {
		_, def, err := c.Libraries(ctx)
		if err != nil {
			return 0, err
		}
		lib = def
	}
	path := "/cdb/add-book/0/n/" + url.PathEscape(filename) + "/" + url.PathEscape(lib)
	raw, err := c.doType(ctx, http.MethodPost, path, data, "application/octet-stream")
	if err != nil {
		return 0, err
	}
	var out struct {
		BookID int `json:"book_id"`
	}
	if json.Unmarshal(raw, &out) != nil || out.BookID == 0 {
		return 0, errors.New("calibre didn't say which book it made (it may have refused a duplicate)")
	}
	return out.BookID, nil
}
