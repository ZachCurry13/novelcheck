package libby

import (
	"context"
	"net/url"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Found is one book a library lends, from a free-text search: it need not be in NovelCheck.
type Found struct {
	Title  string        `json:"title"`
	Author string        `json:"author"`
	Cover  string        `json:"cover,omitempty"`
	Key    string        `json:"key"` // title+author, to match the same book across libraries
	Ebook  *Availability `json:"ebook,omitempty"`
	Audio  *Availability `json:"audio,omitempty"`
}

type searchItem struct {
	item
	Covers map[string]struct {
		Href string `json:"href"`
	} `json:"covers"`
}

// Search looks for books by any words (title, author, series) at one library, for one kind of media.
func (c *Client) Search(ctx context.Context, key, query, media string) ([]Found, error) {
	if media != "audiobook" {
		media = "ebook"
	}
	var res struct {
		Items []searchItem `json:"items"`
	}
	q := url.Values{"query": {strings.TrimSpace(query)}, "maxItems": {"15"}, "mediaTypes": {media}}
	if err := c.get(ctx, "/libraries/"+url.PathEscape(key)+"/media?"+q.Encode(), &res); err != nil {
		return nil, err
	}
	out := make([]Found, 0, len(res.Items))
	for _, it := range res.Items {
		a := &Availability{Found: true, Available: it.Available, Holdable: it.Holdable, Copies: it.Owned, Free: it.Free,
			Holds: it.Holds, WaitDays: it.Wait, URL: "https://" + key + ".overdrive.com/media/" + it.ID}
		for _, f := range it.Formats {
			a.Kindle = a.Kindle || f.ID == "ebook-kindle"
		}
		f := Found{Title: it.Title, Author: it.Creator, Key: store.NormKey(it.Title, it.Creator)}
		for _, name := range []string{"cover150Wide", "cover300Wide", "cover510Wide"} {
			if h := it.Covers[name].Href; h != "" {
				f.Cover = h
				break
			}
		}
		if media == "audiobook" {
			f.Audio = a
		} else {
			f.Ebook = a
		}
		out = append(out, f)
	}
	return out, nil
}
