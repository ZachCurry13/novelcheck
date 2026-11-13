package events

// Pages that link to book lists rather than books: a Linktree of the day's
// free books ("Kids", "Mystery", "Thrillers"), each an Amazon wish list.
// NovelCheck offers the lists, and reads the ones picked into one event.

import (
	"context"
	"errors"
	"html"
	"regexp"
	"strings"
	"sync"
	"time"
)

// MaxLists caps the lists read for one event (each takes a few seconds).
const MaxLists = 15

var wishRe = regexp.MustCompile(`(?i)^https?://(?:www\.|smile\.)?amazon\.[a-z]{2,3}(?:\.[a-z]{2})?/(?:hz/wishlist/ls|gp/registry/wishlist|registry/wishlist)/([A-Z0-9]+)`)

// List is one Amazon list a page links to. Picked: the address given
// pointed at it (a shared Linktree link).
type List struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Picked bool   `json:"picked"`
}

// Lists finds the Amazon wish lists a page links to, named by their link.
func Lists(page, focus string) []List {
	var out []List
	seen := map[string]bool{}
	mark := -1
	if focus != "" {
		mark = strings.Index(page, `id="`+focus+`"`)
	}
	for _, m := range anchorRe.FindAllStringSubmatchIndex(page, -1) {
		attrs, inner := page[m[2]:m[3]], page[m[4]:m[5]]
		hm := hrefRe.FindStringSubmatch(attrs)
		if hm == nil {
			continue
		}
		link := html.UnescapeString(hm[1])
		wm := wishRe.FindStringSubmatch(link)
		if wm == nil || seen[strings.ToUpper(wm[1])] || len(out) >= 50 {
			continue
		}
		seen[strings.ToUpper(wm[1])] = true
		name := text(inner)
		if am := attrRe.FindStringSubmatch(attrs); am != nil {
			name = text(am[1]) // Linktree names the link here; its text may carry pictures
		}
		if name == "" {
			name = "Amazon list " + wm[1]
		}
		picked := mark >= 0 && m[0] > mark && m[0]-mark < 1500
		if picked {
			mark = -1 // only the first list after the mark is the shared one
		}
		out = append(out, List{Name: name, URL: link, Picked: picked})
	}
	return out
}

// FetchLists reads the picked Amazon lists, a few at a time, as one page.
// It refuses anything but Amazon wish lists.
func FetchLists(ctx context.Context, links []string) (string, error) {
	if len(links) > MaxLists {
		return "", errors.New("pick up to 15 lists at a time")
	}
	for _, l := range links {
		if !wishRe.MatchString(strings.TrimSpace(l)) {
			return "", errors.New("only Amazon lists can be read this way")
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second) // under the tunnel's 100 s
	defer cancel()
	pages := make([]string, len(links))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, l := range links {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if p, err := Fetch(ctx, l); err == nil {
				pages[i] = p
			}
		}()
	}
	wg.Wait()
	read := 0
	for _, p := range pages {
		if p != "" {
			read++
		}
	}
	if read == 0 {
		return "", errors.New("Amazon didn't answer for those lists; try again in a minute")
	}
	return strings.Join(pages, "\n"), nil
}
