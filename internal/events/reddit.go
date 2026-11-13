package events

// "Free right now": the books r/FreeEBOOKS posted in the last few days,
// kept as one pinned event that is refreshed every few hours.

import (
	"context"
	"html"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/zachcurry13/novelcheck/internal/store"
)

var redditFeed = "https://www.reddit.com/r/FreeEBOOKS/new/.rss?limit=100" // a test points it at its own feed

const (
	redditPage = "https://www.reddit.com/r/FreeEBOOKS/"
	freeName   = "Free right now"
	freeDays   = 4 // a post older than this is likely no longer free
)

var (
	entryRe     = regexp.MustCompile(`(?s)<entry>(.*?)</entry>`)
	eTitleRe    = regexp.MustCompile(`(?s)<title[^>]*>(.*?)</title>`)
	eWhenRe     = regexp.MustCompile(`<(?:published|updated)>([^<]+)</`)
	eBodyRe     = regexp.MustCompile(`(?s)<content[^>]*>(.*?)</content>`)
	bracketRe   = regexp.MustCompile(`\[[^\]]*\]|\((?:[^)]*\d[^)]*|[^)]*(?i:free|kindle|kU|unlimited)[^)]*)\)`)
	symbolRe    = regexp.MustCompile(`[\p{So}\p{Sk}]+`)
	authorCut   = regexp.MustCompile(`\s+(?:—|–|-|by)\s+`)
	offerRe     = regexp.MustCompile(`(?i)\b(?:free|e-?book|kindle|unlimited|\d{1,2}(?:st|nd|rd|th)?)\b`)
	promoOnlyRe = regexp.MustCompile(`(?i)^(?:free|kindle|e-?books?|books?|romance|fantasy|mystery|thriller|today|now|\W)+$`)
	promoRe     = regexp.MustCompile(`(?i)^(?:free\b[^:]*|kindle\b[^:]*|new\b[^:]*)[:!]\s*`)
)

// freeRightNow refreshes the standing event from the feed.
func freeRightNow(ctx context.Context, st *store.Store, enqueue func(bool, ...int64), now time.Time) error {
	feed, err := get(ctx, redditFeed, "application/atom+xml, application/xml;q=0.9")
	if err != nil {
		return err
	}
	books := redditBooks(feed, now)
	if len(books) == 0 {
		return nil
	}
	id, archived, found := st.EventBySource(redditPage)
	if archived {
		return nil // someone put it away: leave it there
	}
	if !found {
		if id, _, err = st.CreateEvent(freeName, redditPage, byApp, "", nil); err != nil {
			return err
		}
		_ = st.PinEventOn(id)
	}
	unrated, err := st.ReplaceEventBooks(id, books)
	if len(unrated) > 0 {
		enqueue(false, unrated...) // in line with the rest: these come every day
	}
	return err
}

// redditBooks reads the feed's recent posts that link to a book on Amazon.
func redditBooks(feed string, now time.Time) []store.EventEntry {
	var out []store.EventEntry
	seen := map[string]bool{}
	for _, m := range entryRe.FindAllStringSubmatch(feed, -1) {
		e := m[1]
		if w := eWhenRe.FindStringSubmatch(e); w != nil {
			if t, err := time.Parse(time.RFC3339, strings.TrimSpace(w[1])); err == nil && now.Sub(t) > freeDays*24*time.Hour {
				continue
			}
		}
		body := ""
		if b := eBodyRe.FindStringSubmatch(e); b != nil {
			body = html.UnescapeString(b[1])
		}
		found := postLinks(body, seen)
		if len(found) == 1 { // one book: the post's title names it better than its link
			if t := eTitleRe.FindStringSubmatch(e); t != nil {
				if title, author := postTitle(html.UnescapeString(html.UnescapeString(t[1]))); !promoOnlyRe.MatchString(title) {
					found[0].Title, found[0].Author = title, author
				}
			}
		}
		for _, b := range found {
			b.Title = strings.Trim(b.Title, `"'“”‘’ `)
			if !namesABook(b.Title) {
				continue // a bare link, a store ("US") or a date: nothing to name the book by
			}
			seen[b.ASIN] = true
			out = append(out, b)
			if len(out) >= MaxBooks {
				return out
			}
		}
	}
	return out
}

// postTitle takes the book's title and author out of a post's title, such as
// "[Kindle] The Island of Silent Answers — Meikel Morozov [Free until: November 17]".
func postTitle(s string) (title, author string) {
	s = symbolRe.ReplaceAllString(bracketRe.ReplaceAllString(s, " "), " ")
	if i := strings.LastIndex(s, "]"); i >= 0 {
		s = s[i+1:] // "[[Free Kindle eBook] Title": a bracket left over
	}
	s = strings.TrimSpace(promoRe.ReplaceAllString(strings.TrimSpace(s), ""))
	s = strings.Join(strings.Fields(s), " ")
	if parts := authorCut.Split(s, 2); len(parts) == 2 {
		title, author = strings.TrimSpace(parts[0]), strings.Trim(strings.TrimSpace(parts[1]), ".,;:")
		if offerRe.MatchString(author) {
			return title, "" // "Title - ebook free on November 12-14"
		}
		if r := []rune(author); len(r) == 0 || !unicode.IsUpper(r[0]) {
			return s, "" // "Seen by the Mountain Man" is one title
		}
		if len(strings.Fields(author)) > 3 || strings.ContainsAny(author, "/|:") {
			author = "" // a subtitle or a genre, not a name ("Job Interview Ghosting Guide")
		}
		return title, author
	}
	return s, ""
}

var notTitleRe = regexp.MustCompile(`(?i)^(?:[a-z]{2}|amazon\S*|kindle|link|here|free|https?:.*|/.*|[\d/.\-\s]+)$`)

// namesABook: link text a person would read as a book's name.
func namesABook(t string) bool {
	letters := 0
	for _, r := range t {
		if unicode.IsLetter(r) {
			letters++
		}
	}
	return letters >= 3 && !notTitleRe.MatchString(strings.TrimSpace(t)) && !strings.Contains(strings.ToLower(t), "amazon.")
}

// postLinks are the books a post links to on Amazon, once each (one book
// linked for each country's store is one book), named by their link text
// where it names them.
func postLinks(body string, seen map[string]bool) []store.EventEntry {
	names := map[string]string{}
	for _, b := range fromHTML(body) {
		if b.ASIN != "" && names[b.ASIN] == "" {
			names[b.ASIN] = b.Title
		}
	}
	var out []store.EventEntry
	inPost := map[string]bool{}
	site := amazonSite(body)
	for _, m := range anchorRe.FindAllStringSubmatch(body, -1) {
		h := hrefRe.FindStringSubmatch(m[1])
		if h == nil {
			continue
		}
		link, ok := bookLink(html.UnescapeString(h[1]), site)
		a := asinRe.FindStringSubmatch(link)
		if !ok || a == nil {
			continue
		}
		asin := strings.ToUpper(a[1])
		if seen[asin] || inPost[asin] {
			continue
		}
		inPost[asin] = true
		out = append(out, store.EventEntry{Title: names[asin], ASIN: asin, Link: link})
	}
	return out
}
