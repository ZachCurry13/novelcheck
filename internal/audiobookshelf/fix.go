package audiobookshelf

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// Fix is what the cleanup (api/absclean.go) changes on one audiobook: only the fields that are set.
type Fix struct {
	Title       string   `json:"title,omitempty"`
	Author      string   `json:"author,omitempty"`
	Series      string   `json:"series,omitempty"`
	Sequence    string   `json:"sequence,omitempty"`
	ISBN        string   `json:"isbn,omitempty"`
	ASIN        string   `json:"asin,omitempty"`
	Narrators   []string `json:"narrators,omitempty"`
	Genres      []string `json:"genres,omitempty"`
	Description string   `json:"description,omitempty"`
	Cover       string   `json:"cover,omitempty"` // a picture address for Audiobookshelf to fetch
}

// Empty says whether the fix changes nothing.
func (f Fix) Empty() bool {
	return f.Title == "" && f.Author == "" && f.Series == "" && f.ISBN == "" && f.ASIN == "" && len(f.Narrators) == 0 &&
		len(f.Genres) == 0 && f.Description == "" && f.Cover == ""
}

// ErrNoEditRights: the account NovelCheck signs in with may not edit books in Audiobookshelf.
var ErrNoEditRights = errors.New("the Audiobookshelf account NovelCheck uses can't edit books (give it the Update permission, or use an admin's token)")

// ApplyFix writes the fix into the audiobook's details, then sets the cover when there is one.
func (c *Client) ApplyFix(ctx context.Context, itemID string, f Fix) error {
	meta := map[string]any{}
	if f.Title != "" {
		meta["title"] = f.Title
	}
	if f.Author != "" {
		meta["authors"] = []map[string]string{{"name": f.Author}}
	}
	if f.Series != "" {
		meta["series"] = []map[string]string{{"name": f.Series, "sequence": f.Sequence}}
	}
	if f.ISBN != "" {
		meta["isbn"] = f.ISBN
	}
	if f.ASIN != "" {
		meta["asin"] = f.ASIN
	}
	if len(f.Narrators) > 0 {
		meta["narrators"] = f.Narrators
	}
	if len(f.Genres) > 0 {
		meta["genres"] = f.Genres
	}
	if f.Description != "" {
		meta["description"] = f.Description
	}
	path := "/api/items/" + url.PathEscape(itemID)
	if len(meta) > 0 {
		if err := rights(c.JSON(ctx, http.MethodPatch, path+"/media", map[string]any{"metadata": meta}, nil)); err != nil {
			return err
		}
	}
	if f.Cover != "" {
		return rights(c.JSON(ctx, http.MethodPost, path+"/cover", map[string]string{"url": f.Cover}, nil))
	}
	return nil
}

func rights(err error) error {
	if err != nil && (strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "Forbidden")) {
		return ErrNoEditRights
	}
	return err
}
