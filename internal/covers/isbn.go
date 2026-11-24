package covers

import (
	"context"
	"strings"
)

// ISBN10 turns a 978 ISBN-13 into its ISBN-10 ("" for anything else). Amazon uses a print book's ISBN-10 as its
// number, so it has the cover of most print books Open Library doesn't.
func ISBN10(isbn string) string {
	isbn = strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(isbn))
	switch {
	case isbnRE.MatchString(isbn) && len(isbn) == 10:
		return isbn
	case isbnRE.MatchString(isbn) && strings.HasPrefix(isbn, "978"):
		body, sum := isbn[3:12], 0
		for i, r := range body {
			sum += (10 - i) * int(r-'0')
		}
		if c := (11 - sum%11) % 11; c == 10 {
			return body + "X"
		} else {
			return body + string(rune('0'+c))
		}
	}
	return ""
}

// ByISBN looks for the book's cover by an ISBN with the sources that answer by ISBN: Open Library's large then
// medium picture, then Amazon's (by the ISBN-10). A hit is kept as out. The ISBN of a search's matching book often
// has a cover when the one the book carries has none.
func (c Cache) ByISBN(ctx context.Context, isbn, out, miss string) bool {
	isbn = strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(isbn))
	if c.Dir == "" || !isbnRE.MatchString(isbn) {
		return false
	}
	base := c.OpenLibraryURL
	if base == "" {
		base = openLibrary
	}
	for _, size := range []string{"L", "M"} {
		if _, _, ok := c.fetch(ctx, base+"/b/isbn/"+isbn+"-"+size+".jpg?default=false", out, miss); ok {
			return true
		}
	}
	if a := ISBN10(isbn); a != "" {
		amazon := c.AmazonURL
		if amazon == "" {
			amazon = "https://m.media-amazon.com"
		}
		if _, _, ok := c.fetch(ctx, amazon+"/images/P/"+a+".01.LZZZZZZZ.jpg", out, miss); ok {
			return true
		}
	}
	return false
}
