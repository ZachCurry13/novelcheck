package covers

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
)

// asinRE is a Kindle book's Amazon number (B0 and eight letters or digits) or a print book's ISBN-10, which
// Amazon uses as the ASIN.
var asinRE = regexp.MustCompile(`^(B0[0-9A-Z]{8}|\d{9}[\dX])$`)

// Amazon saves Amazon's own cover picture for asin as name.jpg. Kindle-only books (most free Kindle books) have
// no ISBN, and Open Library and Google Books don't know them, but Amazon always has their cover. For a number it
// doesn't know Amazon answers with a 1-pixel picture, which fetch turns down (too small to be a cover).
func (c Cache) Amazon(ctx context.Context, name, asin string) bool {
	asin = strings.ToUpper(strings.TrimSpace(asin))
	if c.Dir == "" || !storedName.MatchString(name) || !asinRE.MatchString(asin) {
		return false
	}
	base := c.AmazonURL
	if base == "" {
		base = "https://m.media-amazon.com"
	}
	out := filepath.Join(c.Dir, name+".jpg")
	if _, ok := c.have(out); ok {
		return true
	}
	_, _, ok := c.fetch(ctx, base+"/images/P/"+asin+".01._SCLZZZZZZZ_.jpg", out, out+".none")
	if ok {
		c.Forget(name)
	}
	return ok
}

// ASINsIn picks the Amazon numbers out of file names and ids ("Title_B00ABC1234.kfx", "B00ABC1234_EBOK.azw").
func ASINsIn(texts ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range texts {
		for _, m := range asinInText.FindAllStringSubmatch(strings.ToUpper(t), -1) {
			if a := m[1]; !seen[a] {
				seen[a] = true
				out = append(out, a)
			}
		}
	}
	return out
}

var asinInText = regexp.MustCompile(`(?:^|[^0-9A-Z])(B0[0-9A-Z]{8})(?:[^0-9A-Z]|$)`)
