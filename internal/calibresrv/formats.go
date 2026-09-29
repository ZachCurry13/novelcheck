package calibresrv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// RemoveFormat removes one format of a book; calibre moves the file to its
// recycle bin (Remove books → Restore recently deleted undoes it).
func (c *Client) RemoveFormat(ctx context.Context, id int, format string) error {
	_, err := c.cmd(ctx, "remove_format", []any{id, strings.ToUpper(format)})
	return err
}

// AddFormat adds a file as a format of a book (for putting back a removed
// one). The file travels as msgpack, since JSON can't carry it.
func (c *Client) AddFormat(ctx context.Context, id int, format, name string, data []byte) error {
	body, err := msgpack([]any{id, []any{name, data}, strings.ToUpper(format), false})
	if err != nil {
		return err
	}
	raw, err := c.doType(ctx, http.MethodPost, "/cdb/cmd/add_format/0"+c.libraryQuery(), body, "application/x-msgpack")
	if err != nil {
		return err
	}
	var out struct {
		Result json.RawMessage `json:"result"`
		Err    string          `json:"err"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return errors.New("unexpected answer from calibre")
	}
	if out.Err != "" {
		return fmt.Errorf("calibre: %s", out.Err)
	}
	return nil
}

func (c *Client) libraryQuery() string {
	if c.Library == "" {
		return ""
	}
	return "?library_id=" + url.QueryEscape(c.Library)
}

// StartConversion asks calibre to convert a book's format to EPUB (with its
// default settings) and returns the job's id. Calibre adds the EPUB when done.
func (c *Client) StartConversion(ctx context.Context, id int, from string) (int, error) {
	body, _ := json.Marshal(map[string]any{"input_fmt": strings.ToUpper(from), "output_fmt": "EPUB", "options": map[string]any{}})
	raw, err := c.do(ctx, http.MethodPost, "/conversion/start/"+strconv.Itoa(id)+c.libraryQuery(), body)
	if err != nil {
		return 0, err
	}
	var job int
	if err := json.Unmarshal(raw, &job); err != nil {
		return 0, errors.New("unexpected answer from calibre's converter")
	}
	return job, nil
}

// Conversion is how a conversion job is going.
type Conversion struct {
	Running bool    `json:"running"`
	OK      bool    `json:"ok"`
	Percent float64 `json:"percent"`
	Msg     string  `json:"msg"`
	Log     string  `json:"log"`
	Trace   string  `json:"traceback"`
}

// ConversionStatus asks how a conversion job is going.
func (c *Client) ConversionStatus(ctx context.Context, job int) (Conversion, error) {
	raw, err := c.do(ctx, http.MethodPost, "/conversion/status/"+strconv.Itoa(job)+c.libraryQuery(), []byte("{}"))
	if err != nil {
		return Conversion{}, err
	}
	var st Conversion
	if err := json.Unmarshal(raw, &st); err != nil {
		return Conversion{}, errors.New("unexpected answer from calibre's converter")
	}
	return st, nil
}
