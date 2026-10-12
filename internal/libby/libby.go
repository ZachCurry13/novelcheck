// Package libby checks whether a library lends a book, through OverDrive's
// public search (the service behind Libby; no key or login): copies, holds
// and the estimated wait. It can't place holds: that needs the person signed
// in to Libby, so NovelCheck links to the book at the library's own page.
package libby

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Base is OverDrive's public API; tests point Client.Base elsewhere.
const Base = "https://thunder.api.overdrive.com/v2"

var keyRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,40}$`)

// CleanKey reads a library key: "lapl", or the address "lapl.overdrive.com".
func CleanKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	s, _, _ = strings.Cut(s, "/")
	s = strings.TrimSuffix(s, ".overdrive.com")
	if keyRE.MatchString(s) {
		return s
	}
	return ""
}

// Availability is what a library says about one book.
type Availability struct {
	Found     bool   `json:"found"`
	Available bool   `json:"available"`
	Holdable  bool   `json:"holdable"`
	Copies    int    `json:"copies"`
	Free      int    `json:"free"`
	Holds     int    `json:"holds"`
	WaitDays  int    `json:"wait_days"`
	URL       string `json:"url"`
	Kindle    bool   `json:"kindle"`
}

// Client asks OverDrive; answers are kept for a few hours.
type Client struct {
	Base string
	HTTP *http.Client

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	a  Availability
	at time.Time
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	base := c.Base
	if base == "" {
		base = Base
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return err
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("can't reach the library service: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return errors.New("no library with that key")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the library service answered %s", resp.Status)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out)
}

// LibraryName checks a key and returns the library's name.
func (c *Client) LibraryName(ctx context.Context, key string) (string, error) {
	var l struct {
		Name string `json:"name"`
	}
	if err := c.get(ctx, "/libraries/"+url.PathEscape(key), &l); err != nil {
		return "", err
	}
	return l.Name, nil
}

type item struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Creator   string `json:"firstCreatorName"`
	Available bool   `json:"isAvailable"`
	Holdable  bool   `json:"isHoldable"`
	Owned     int    `json:"ownedCopies"`
	Free      int    `json:"availableCopies"`
	Holds     int    `json:"holdsCount"`
	Wait      int    `json:"estimatedWaitDays"`
	Formats   []struct {
		ID string `json:"id"`
	} `json:"formats"`
}

// Check looks a book up at a library (e-books only) by title and author.
func (c *Client) Check(ctx context.Context, key, title, author string) (Availability, error) {
	ck := key + "|" + store.NormKey(title, author)
	c.mu.Lock()
	if v, ok := c.cache[ck]; ok && time.Since(v.at) < 3*time.Hour {
		c.mu.Unlock()
		return v.a, nil
	}
	c.mu.Unlock()
	var res struct {
		Items []item `json:"items"`
	}
	q := url.Values{"query": {strings.TrimSpace(title + " " + author)}, "maxItems": {"12"}, "mediaTypes": {"ebook"}}
	if err := c.get(ctx, "/libraries/"+url.PathEscape(key)+"/media?"+q.Encode(), &res); err != nil {
		return Availability{}, err
	}
	want := store.NormKey(title, author)
	var best *item
	for i := range res.Items {
		if store.NormKey(res.Items[i].Title, res.Items[i].Creator) == want {
			best = &res.Items[i]
			break
		}
	}
	a := Availability{}
	if best != nil {
		a = Availability{Found: true, Available: best.Available, Holdable: best.Holdable, Copies: best.Owned, Free: best.Free,
			Holds: best.Holds, WaitDays: best.Wait, URL: "https://" + key + ".overdrive.com/media/" + best.ID}
		for _, f := range best.Formats {
			a.Kindle = a.Kindle || f.ID == "ebook-kindle"
		}
	}
	c.mu.Lock()
	if c.cache == nil {
		c.cache = map[string]cached{}
	}
	c.cache[ck] = cached{a, time.Now()}
	c.mu.Unlock()
	return a, nil
}
