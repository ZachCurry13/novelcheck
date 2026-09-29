package calibresrv

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// Titles returns calibre's title for each of ids (ids calibre doesn't have
// are simply absent). Used to check the Content server is serving the same
// library NovelCheck reads before anything is removed.
func (c *Client) Titles(ctx context.Context, ids []int) (map[int]string, error) {
	if len(ids) == 0 {
		return map[int]string{}, nil
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = "id:" + strconv.Itoa(id)
	}
	// list(fields, sort_by, ascending, search_text, limit)
	raw, err := c.cmd(ctx, "list", []any{[]string{"title"}, "id", true, strings.Join(parts, " or "), -1})
	if err != nil {
		return nil, err
	}
	var res struct {
		Data struct {
			Title map[string]string `json:"title"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, errors.New("unexpected answer from calibre's book list")
	}
	out := make(map[int]string, len(res.Data.Title))
	for k, v := range res.Data.Title {
		if id, err := strconv.Atoi(k); err == nil {
			out[id] = v
		}
	}
	return out, nil
}

// Book is what calibre has for a book: its title and formats (EPUB, MOBI…).
type Book struct {
	Title   string
	Formats []string
}

// Books returns the title and formats of each of ids calibre has, so format
// cleanup can check a kept format is really there before removing others.
func (c *Client) Books(ctx context.Context, ids []int) (map[int]Book, error) {
	out := map[int]Book{}
	for start := 0; start < len(ids); start += 200 {
		part := ids[start:min(start+200, len(ids))]
		terms := make([]string, len(part))
		for i, id := range part {
			terms[i] = "id:" + strconv.Itoa(id)
		}
		raw, err := c.cmd(ctx, "list", []any{[]string{"title", "formats"}, "id", true, strings.Join(terms, " or "), -1})
		if err != nil {
			return nil, err
		}
		var res struct {
			Data struct {
				Title   map[string]string   `json:"title"`
				Formats map[string][]string `json:"formats"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return nil, errors.New("unexpected answer from calibre's book list")
		}
		for k, title := range res.Data.Title {
			id, err := strconv.Atoi(k)
			if err != nil {
				continue
			}
			b := Book{Title: title}
			for _, f := range res.Data.Formats[k] { // names, or file paths on some versions
				if i := strings.LastIndexAny(f, "./\\"); i >= 0 && f[i] == '.' {
					f = f[i+1:]
				}
				b.Formats = append(b.Formats, strings.ToUpper(f))
			}
			out[id] = b
		}
	}
	return out, nil
}

// Has reports whether the book has the format.
func (b Book) Has(format string) bool {
	for _, f := range b.Formats {
		if strings.EqualFold(f, format) {
			return true
		}
	}
	return false
}

// Remove moves the books to calibre's recycle bin (permanent=false), exactly
// like selecting them in calibre and pressing Delete.
func (c *Client) Remove(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := c.cmd(ctx, "remove", []any{ids, false})
	return err
}

// SetMetadata changes a book's title and, when series is given, its series
// and number: the same as `calibredb set_metadata --field`. Calibre renames
// the book's folder and files to match. found is false if calibre has no
// book with that id.
func (c *Client) SetMetadata(ctx context.Context, id int, title, series string, index float64) (found bool, err error) {
	fields := [][]any{{"title", title}}
	if series != "" { // the number goes last: calibre sets it on the series just given
		fields = append(fields, []any{"series", series}, []any{"series_index", index})
	}
	raw, err := c.cmd(ctx, "set_metadata", []any{"fields", id, fields})
	if err != nil {
		return false, err
	}
	return len(raw) > 0 && string(raw) != "null", nil
}
