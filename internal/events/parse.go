// Package events reads the book lists of Stuff Your Kindle events (and
// similar free e-book promotions) from pasted text or HTML, or from the
// event's page, and cleans up old events.
package events

import (
	"html"
	"regexp"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// MaxBooks caps one event's list.
const MaxBooks = 500

var (
	anchorRe  = regexp.MustCompile(`(?is)<a\b([^>]*)>(.*?)</a>`)
	hrefRe    = regexp.MustCompile(`(?is)\bhref\s*=\s*["']([^"']+)["']`)
	attrRe    = regexp.MustCompile(`(?is)\b(?:title|aria-label|alt)\s*=\s*["']([^"']{3,300})["']`)
	asinRe    = regexp.MustCompile(`(?i)/(?:dp|gp/product|gp/aw/d|d)/([A-Z0-9]{10})(?:[/?#]|$)`)
	headingRe = regexp.MustCompile(`(?is)<h[1-5][^>]*>(.*?)</h[1-5]>`)
	tagRe     = regexp.MustCompile(`(?s)<[^>]*>`)
	spaceRe   = regexp.MustCompile(`\s+`)
	lineNoise = regexp.MustCompile(`(?i)^[\s\-•*·\d.)]+|\s*[(\[]?(free|\$\s?0\.00|0,00\s?€|kindle edition|ebook)[)\]]?\s*$`)
	// Link texts that say what to do, not which book.
	genericRe = regexp.MustCompile(`(?i)^(buy|get|grab|claim|download|free|view|see|shop|read|amazon|kindle|click|here|more|link|now|on|it|the|book|this|for|at|\W)+$`)
)

// Parse reads the books in pasted HTML (links to Amazon give the ASIN; the
// link text, or else the heading before it, the title) and pasted text
// (lines like "Title by Author"). Repeats are dropped.
func Parse(htmlText, plain string) []store.EventEntry {
	var out []store.EventEntry
	seen := map[string]bool{}
	add := func(e store.EventEntry) {
		e.Title = strings.TrimSpace(e.Title)
		if t, a, ok := strings.Cut(e.Title, " by "); ok && e.Author == "" && len(a) < 80 {
			e.Title, e.Author = strings.TrimSpace(t), strings.TrimSpace(a)
		}
		key := strings.ToLower(e.Title)
		if e.ASIN != "" {
			key = e.ASIN
		}
		if len([]rune(e.Title)) < 2 || seen[key] || len(out) >= MaxBooks {
			return
		}
		seen[key], seen[strings.ToLower(e.Title)] = true, true
		out = append(out, e)
	}
	if strings.TrimSpace(htmlText) != "" {
		for _, e := range fromHTML(htmlText) {
			add(e)
		}
	}
	if len(out) == 0 {
		for _, line := range strings.Split(plain, "\n") {
			if t := strings.TrimSpace(lineNoise.ReplaceAllString(line, "")); t != "" && !genericRe.MatchString(t) {
				add(store.EventEntry{Title: t})
			}
		}
	}
	return out
}

func fromHTML(page string) []store.EventEntry {
	var out []store.EventEntry
	for _, m := range anchorRe.FindAllStringSubmatchIndex(page, -1) {
		attrs, inner := page[m[2]:m[3]], page[m[4]:m[5]]
		hm := hrefRe.FindStringSubmatch(attrs)
		if hm == nil {
			continue
		}
		link := html.UnescapeString(hm[1])
		if !isAmazon(link) {
			continue
		}
		e := store.EventEntry{Link: link}
		if am := asinRe.FindStringSubmatch(link); am != nil {
			e.ASIN = strings.ToUpper(am[1])
		}
		title := text(inner)
		if genericRe.MatchString(title) || title == "" {
			title = ""
			if am := attrRe.FindStringSubmatch(inner + " " + attrs); am != nil && !genericRe.MatchString(html.UnescapeString(am[1])) {
				title = html.UnescapeString(am[1])
			} else if hs := headingRe.FindAllStringSubmatch(page[max(0, m[0]-1500):m[0]], -1); len(hs) > 0 {
				title = text(hs[len(hs)-1][1]) // the nearest heading before the button
			}
		}
		if title != "" {
			e.Title = title
			out = append(out, e)
		}
	}
	return out
}

func isAmazon(link string) bool {
	l := strings.ToLower(link)
	return strings.Contains(l, "amazon.") || strings.Contains(l, "amzn.to/") || strings.Contains(l, "a.co/")
}

// text is HTML as plain words.
func text(s string) string {
	return strings.TrimSpace(spaceRe.ReplaceAllString(html.UnescapeString(tagRe.ReplaceAllString(s, " ")), " "))
}
