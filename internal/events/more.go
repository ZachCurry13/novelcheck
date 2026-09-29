package events

// Pages that show more books on request ("Load more books") or split their
// list over pages: Fetch asks for the rest the way the page itself would, so
// it finds every book, not just the first few.

import (
	"context"
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// A "Load more" button or link, with its attributes.
	moreRe = regexp.MustCompile(`(?is)<(?:button|a)\b([^>]*)>(?:\s|<[^>]*>)*(?:load|show|see|view)\s+more\b`)
	attrKV = regexp.MustCompile(`(?is)\b(data-[a-z-]+|href)\s*=\s*["']([^"']*)["']`)
	nextRe = regexp.MustCompile(`(?is)<(?:link|a)\b[^>]*\brel\s*=\s*["']next["'][^>]*>`)
)

const (
	maxMore  = 60       // extra requests for one list
	maxTotal = 16 << 20 // everything they bring
)

// loadMore returns the books the page leaves for "Load more" or next pages.
func loadMore(ctx context.Context, page *url.URL, body string) string {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if m := moreRe.FindStringSubmatch(body); m != nil {
		a := attrs(m[1])
		if start, key := firstOf(a, "data-offset"), "offset"; start != "" || firstOf(a, "data-page", "data-next-page") != "" {
			if start == "" {
				start, key = firstOf(a, "data-page", "data-next-page"), "page"
			}
			endpoint := firstOf(a, "data-url", "data-href", "data-endpoint", "data-source")
			if endpoint == "" {
				endpoint = strings.TrimSuffix(page.Path, "/") + "/books" // how Stuff Your Kindle's pages load more
			}
			n, err := strconv.Atoi(start)
			if u := sameSite(page, endpoint); u != nil && err == nil && n > 0 {
				return byCount(ctx, u, key, n)
			}
			return ""
		}
	}
	return followLinks(ctx, page, body)
}

// byCount asks for the next books by offset or page number until the site
// says there are no more.
func byCount(ctx context.Context, endpoint *url.URL, key string, n int) string {
	step := 1
	if key == "offset" {
		step = n // the first offset is how many the page shows
	}
	var out strings.Builder
	for i := 0; i < maxMore && out.Len() < maxTotal; i++ {
		u := *endpoint
		q := u.Query()
		q.Set(key, strconv.Itoa(n))
		u.RawQuery = q.Encode()
		body, err := get(ctx, u.String(), "application/json, text/html;q=0.9")
		if err != nil {
			break
		}
		c := readChunk(body)
		if len(fromHTML(c.html)) == 0 {
			break
		}
		out.WriteString(c.html)
		if c.more != nil && !*c.more {
			break
		}
		if c.next > n {
			n = c.next
		} else {
			n += step
		}
	}
	return out.String()
}

// followLinks reads the next pages (rel="next", or a "Load more" link).
func followLinks(ctx context.Context, page *url.URL, body string) string {
	var out strings.Builder
	seen := map[string]bool{page.String(): true}
	base := page
	for i := 0; i < maxMore && out.Len() < maxTotal; i++ {
		link := nextLink(base, body)
		if link == nil || seen[link.String()] {
			break
		}
		seen[link.String()] = true
		b, err := get(ctx, link.String(), "text/html,application/xhtml+xml")
		if err != nil {
			break
		}
		c := readChunk(b)
		if len(fromHTML(c.html)) == 0 {
			break
		}
		out.WriteString(c.html)
		body, base = c.html, link
	}
	return out.String()
}

// nextLink is where the rest of the list is, on the same site.
func nextLink(base *url.URL, body string) *url.URL {
	if m := nextRe.FindString(body); m != "" {
		return sameSite(base, attrs(m)["href"])
	}
	if m := moreRe.FindStringSubmatch(body); m != nil {
		return sameSite(base, firstOf(attrs(m[1]), "data-url", "data-href", "data-next", "data-next-url", "href"))
	}
	return nil
}

// sameSite resolves a link on the page's own site; others are ignored.
func sameSite(base *url.URL, link string) *url.URL {
	link = strings.TrimSpace(link)
	if link == "" || strings.HasPrefix(link, "#") || strings.HasPrefix(strings.ToLower(link), "javascript:") {
		return nil
	}
	u, err := base.Parse(link)
	if err != nil || u.Host != base.Host || (u.Scheme != "http" && u.Scheme != "https") {
		return nil
	}
	u.Fragment = ""
	return u
}

func attrs(tag string) map[string]string {
	out := map[string]string{}
	for _, m := range attrKV.FindAllStringSubmatch(tag, -1) {
		out[strings.ToLower(m[1])] = html.UnescapeString(m[2])
	}
	return out
}

func firstOf(a map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(a[k]); v != "" {
			return v
		}
	}
	return ""
}

// chunk is one answer: the books' HTML, and what the site said is left.
type chunk struct {
	html string
	more *bool // whether the site says more books are left
	next int   // the next offset or page it gave, or 0
}

// readChunk reads HTML, or JSON carrying HTML ({"html": …, "hasMore": …,
// "nextOffset": …}) as sites' "Load more" answers do.
func readChunk(body string) chunk {
	t := strings.TrimSpace(body)
	var m map[string]any
	if !strings.HasPrefix(t, "{") || json.Unmarshal([]byte(t), &m) != nil {
		return chunk{html: body}
	}
	var c chunk
	for _, k := range []string{"html", "content", "data", "books"} {
		if s, ok := m[k].(string); ok {
			c.html = s
			break
		}
	}
	for _, k := range []string{"hasMore", "has_more", "more"} {
		if b, ok := m[k].(bool); ok {
			c.more = &b
			break
		}
	}
	for _, k := range []string{"nextOffset", "next_offset", "nextPage", "next_page"} {
		if f, ok := m[k].(float64); ok {
			c.next = int(f)
			break
		}
	}
	return c
}
