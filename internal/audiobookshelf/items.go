package audiobookshelf

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Library is one Audiobookshelf library.
type Library struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MediaType string `json:"mediaType"` // "book" or "podcast"
}

// Libraries lists the server's libraries.
func (c *Client) Libraries(ctx context.Context) ([]Library, error) {
	var out struct {
		Libraries []Library `json:"libraries"`
	}
	return out.Libraries, c.JSON(ctx, http.MethodGet, "/api/libraries", nil, &out)
}

var (
	seriesRE = regexp.MustCompile(`^(.*?)\s*#\s*([0-9]+(?:\.[0-9]+)?)`)
	tagRE    = regexp.MustCompile(`<[^>]*>`)
)

type rawItem struct {
	ID    string `json:"id"`
	Media struct {
		Duration float64 `json:"duration"`
		Metadata struct {
			Title       string   `json:"title"`
			Subtitle    string   `json:"subtitle"`
			AuthorName  string   `json:"authorName"`
			Narrator    string   `json:"narratorName"`
			SeriesName  string   `json:"seriesName"`
			Genres      []string `json:"genres"`
			ISBN        string   `json:"isbn"`
			Description string   `json:"description"`
		} `json:"metadata"`
	} `json:"media"`
}

func (r rawItem) item() store.AudioItem {
	m := r.Media.Metadata
	it := store.AudioItem{ID: r.ID, Title: strings.TrimSpace(m.Title), Author: strings.TrimSpace(m.AuthorName), Narrator: strings.TrimSpace(m.Narrator),
		Genres: m.Genres, ISBN: strings.TrimSpace(m.ISBN), DurationSec: int(r.Media.Duration),
		Description: strings.TrimSpace(tagRE.ReplaceAllString(m.Description, " "))}
	if g := seriesRE.FindStringSubmatch(m.SeriesName); g != nil {
		it.Series = g[1]
		fmt.Sscanf(g[2], "%g", &it.SeriesIndex)
	} else {
		it.Series = strings.TrimSpace(strings.SplitN(m.SeriesName, ",", 2)[0])
	}
	return it
}

// Items reads one page (page 0, 1, ...) of a library's audiobooks; total is
// how many the library has.
func (c *Client) Items(ctx context.Context, lib string, page, limit int) (items []store.AudioItem, total int, err error) {
	var out struct {
		Results []rawItem `json:"results"`
		Total   int       `json:"total"`
	}
	q := url.Values{"limit": {fmt.Sprint(limit)}, "page": {fmt.Sprint(page)}, "sort": {"media.metadata.title"}}
	if err := c.JSON(ctx, http.MethodGet, "/api/libraries/"+url.PathEscape(lib)+"/items?"+q.Encode(), nil, &out); err != nil {
		return nil, 0, err
	}
	for _, r := range out.Results {
		if r.ID != "" && r.Media.Metadata.Title != "" && r.Media.Duration > 0 { // ebook-only entries have no audio
			items = append(items, r.item())
		}
	}
	return items, out.Total, nil
}
