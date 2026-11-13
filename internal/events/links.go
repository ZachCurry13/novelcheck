package events

import (
	"net/url"
	"regexp"
	"strings"
)

// Which links on a page are books. Amazon's own pages (a shared wish list)
// link to each book with a short path ("/dp/B0…") and carry hundreds of
// menu and footer links (Careers, Press Center, Sign in) that are not.

var siteRe = regexp.MustCompile(`(?i)https?://www\.amazon\.[a-z]{2,3}(?:\.[a-z]{2})?/`)

// amazonSite is the Amazon store a page belongs to, for its short links.
func amazonSite(page string) string {
	if m := siteRe.FindString(page); m != "" {
		return strings.ToLower(strings.TrimSuffix(m, "/"))
	}
	return "https://www.amazon.com"
}

// bookLink says whether a link points at a book on Amazon, and gives it as a
// full address: a product page (it has an ASIN), a search, or a short link.
func bookLink(link, site string) (string, bool) {
	l := strings.ToLower(strings.TrimSpace(link))
	if strings.HasPrefix(l, "/") && !strings.HasPrefix(l, "//") {
		if asinRe.MatchString(link) {
			return site + strings.TrimSpace(link), true
		}
		return "", false
	}
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil {
		return "", false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	switch {
	case host == "amzn.to" || host == "a.co" || host == "amzn.eu":
		return link, true
	case !strings.HasPrefix(host, "amazon.") && !strings.HasPrefix(host, "smile.amazon."):
		return "", false
	case asinRe.MatchString(u.Path + "/"), u.Path == "/s" || strings.HasPrefix(u.Path, "/s/"):
		return link, true
	}
	return "", false // the store's own pages: menus, help, sign in, careers
}
